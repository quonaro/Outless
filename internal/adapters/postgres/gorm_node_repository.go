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
	GroupID       string    `gorm:"column:group_id"`
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
			Select("id", "url", "group_id", "latency_ms", "status", "country").
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

// ListVLESSURLs returns node URLs for subscription output, filtered by group if specified and latency if configured.
func (r *GormNodeRepository) ListVLESSURLs(ctx context.Context, groupID string) ([]string, error) {
	type row struct {
		URL string `gorm:"column:url"`
	}

	query := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Select("url").
		Where("url LIKE ?", "vless://%").
		Where("status = ?", "healthy")

	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}

	query = query.Order("latency_ms ASC").Limit(50)

	rows := make([]row, 0, 64)
	err := query.Find(&rows).Error
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

// Create inserts a new node into the database.
func (r *GormNodeRepository) Create(ctx context.Context, node domain.Node) error {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		GroupID:   node.GroupID,
		LatencyMS: node.Latency.Milliseconds(),
		Status:    string(node.Status),
		Country:   node.Country,
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("creating node via gorm: %w", err)
	}

	r.logger.Debug("node created", slog.String("node_id", node.ID))
	return nil
}

// FindByID retrieves a node by ID.
func (r *GormNodeRepository) FindByID(ctx context.Context, id string) (domain.Node, error) {
	var model nodeModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Node{}, fmt.Errorf("node not found: %w", domain.ErrNodeNotFound)
		}
		return domain.Node{}, fmt.Errorf("finding node by id: %w", err)
	}

	return domain.Node{
		ID:      model.ID,
		URL:     model.URL,
		GroupID: model.GroupID,
		Latency: time.Duration(model.LatencyMS) * time.Millisecond,
		Status:  domain.NodeStatus(model.Status),
		Country: model.Country,
	}, nil
}

// List returns all nodes.
func (r *GormNodeRepository) List(ctx context.Context) ([]domain.Node, error) {
	var models []nodeModel
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing nodes via gorm: %w", err)
	}

	nodes := make([]domain.Node, 0, len(models))
	for _, model := range models {
		nodes = append(nodes, domain.Node{
			ID:      model.ID,
			URL:     model.URL,
			GroupID: model.GroupID,
			Latency: time.Duration(model.LatencyMS) * time.Millisecond,
			Status:  domain.NodeStatus(model.Status),
			Country: model.Country,
		})
	}

	return nodes, nil
}

// Update updates a node's URL or group.
func (r *GormNodeRepository) Update(ctx context.Context, node domain.Node) error {
	result := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Where("id = ?", node.ID).
		Updates(map[string]any{
			"url":      node.URL,
			"group_id": node.GroupID,
		})

	if result.Error != nil {
		return fmt.Errorf("updating node via gorm: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("node not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Debug("node updated", slog.String("node_id", node.ID))
	return nil
}

// Delete removes a node by ID.
func (r *GormNodeRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&nodeModel{})

	if result.Error != nil {
		return fmt.Errorf("deleting node via gorm: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("node not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Debug("node deleted", slog.String("node_id", id))
	return nil
}
