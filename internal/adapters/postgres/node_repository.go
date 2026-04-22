package postgres

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"time"

	"outless/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NodeRepository persists nodes in PostgreSQL.
type NodeRepository struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewNodeRepository constructs a PostgreSQL-backed node repository.
func NewNodeRepository(pool *pgxpool.Pool, logger *slog.Logger) *NodeRepository {
	return &NodeRepository{pool: pool, logger: logger}
}

// IterateNodes streams nodes from storage using Go iterators.
func (r *NodeRepository) IterateNodes(ctx context.Context) iter.Seq2[domain.Node, error] {
	return func(yield func(domain.Node, error) bool) {
		rows, err := r.pool.Query(ctx, `
			SELECT id, url, latency_ms, status, country
			FROM nodes
		`)
		if err != nil {
			yield(domain.Node{}, fmt.Errorf("querying nodes: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var (
				node      domain.Node
				latencyMS int64
				status    string
			)

			if scanErr := rows.Scan(&node.ID, &node.URL, &latencyMS, &status, &node.Country); scanErr != nil {
				if !yield(domain.Node{}, fmt.Errorf("scanning node row: %w", scanErr)) {
					return
				}
				continue
			}

			node.Latency = time.Duration(latencyMS) * time.Millisecond
			node.Status = domain.NodeStatus(status)

			if !yield(node, nil) {
				return
			}
		}

		if err = rows.Err(); err != nil {
			yield(domain.Node{}, fmt.Errorf("iterating node rows: %w", err))
		}
	}
}

// ListVLESSURLs returns all node URLs for subscription output.
func (r *NodeRepository) ListVLESSURLs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT url
		FROM nodes
		WHERE url LIKE 'vless://%'
	`)
	if err != nil {
		return nil, fmt.Errorf("querying vless urls: %w", err)
	}
	defer rows.Close()

	urls := make([]string, 0, 64)
	for rows.Next() {
		var url string
		if scanErr := rows.Scan(&url); scanErr != nil {
			return nil, fmt.Errorf("scanning vless url: %w", scanErr)
		}
		urls = append(urls, url)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating vless urls: %w", err)
	}

	return urls, nil
}

// UpdateProbeResult updates latest probe metadata for a node.
func (r *NodeRepository) UpdateProbeResult(ctx context.Context, result domain.ProbeResult) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE nodes
		SET latency_ms = $1,
			status = $2,
			country = $3,
			last_checked_at = $4
		WHERE id = $5
	`, result.Latency.Milliseconds(), string(result.Status), result.Country, result.CheckedAt, result.NodeID)
	if err != nil {
		return fmt.Errorf("updating probe result for node %s: %w", result.NodeID, err)
	}

	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("updating probe result for node %s: %w", result.NodeID, domain.ErrNodeNotFound)
	}

	r.logger.Debug("node probe result saved", slog.String("node_id", result.NodeID), slog.String("status", string(result.Status)))
	return nil
}
