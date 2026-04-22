package postgres

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type nodeModel struct {
	ID            string    `gorm:"column:id;primaryKey"`
	URL           string    `gorm:"column:url"`
	LatencyMS     int64     `gorm:"column:latency_ms"`
	Status        string    `gorm:"column:status"`
	Country       string    `gorm:"column:country"`
	LastCheckedAt time.Time `gorm:"column:last_checked_at"`
}

func (nodeModel) TableName() string {
	return "nodes"
}

// GormNodeRepository persists nodes using GORM.
type GormNodeRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGormNodeRepository constructs a GORM-backed node repository.
func NewGormNodeRepository(db *gorm.DB, logger *slog.Logger) *GormNodeRepository {
	return &GormNodeRepository{db: db, logger: logger}
}

// IterateNodes streams nodes from storage using Go iterators.
func (r *GormNodeRepository) IterateNodes(ctx context.Context) iter.Seq2[domain.Node, error] {
	return func(yield func(domain.Node, error) bool) {
		models := make([]nodeModel, 0, 256)
		err := r.db.WithContext(ctx).
			Select("id", "url", "latency_ms", "status", "country").
			Find(&models).Error
		if err != nil {
			yield(domain.Node{}, fmt.Errorf("querying nodes via gorm: %w", err))
			return
		}

		for _, model := range models {
			node := domain.Node{
				ID:      model.ID,
				URL:     model.URL,
				Latency: time.Duration(model.LatencyMS) * time.Millisecond,
				Status:  domain.NodeStatus(model.Status),
				Country: model.Country,
			}

			if !yield(node, nil) {
				return
			}
		}
	}
}

// ListVLESSURLs returns all node URLs for subscription output.
func (r *GormNodeRepository) ListVLESSURLs(ctx context.Context) ([]string, error) {
	type row struct {
		URL string `gorm:"column:url"`
	}

	rows := make([]row, 0, 64)
	err := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Select("url").
		Where("url LIKE ?", "vless://%").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("querying vless urls via gorm: %w", err)
	}

	urls := make([]string, 0, len(rows))
	for _, item := range rows {
		urls = append(urls, item.URL)
	}

	return urls, nil
}

// UpdateProbeResult updates latest probe metadata for a node.
func (r *GormNodeRepository) UpdateProbeResult(ctx context.Context, result domain.ProbeResult) error {
	updates := map[string]any{
		"latency_ms":      result.Latency.Milliseconds(),
		"status":          string(result.Status),
		"country":         result.Country,
		"last_checked_at": result.CheckedAt,
	}

	tx := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Where("id = ?", result.NodeID).
		Updates(updates)
	if tx.Error != nil {
		return fmt.Errorf("updating probe result for node %s via gorm: %w", result.NodeID, tx.Error)
	}

	if tx.RowsAffected == 0 {
		return fmt.Errorf("updating probe result for node %s: %w", result.NodeID, domain.ErrNodeNotFound)
	}

	r.logger.Debug("node probe result saved", slog.String("node_id", result.NodeID), slog.String("status", string(result.Status)))
	return nil
}
