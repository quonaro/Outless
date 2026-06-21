package repository

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type nodeModel struct {
	ID        string    `gorm:"column:id;primaryKey"`
	URL       string    `gorm:"column:url"`
	GroupID   *string   `gorm:"column:group_id;index"`
	Country   string    `gorm:"column:country"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (nodeModel) TableName() string { return "nodes" }

// NodeRepository persists nodes using GORM over SQLite.
type NodeRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewNodeRepository constructs a GORM-backed node repository.
func NewNodeRepository(db *gorm.DB, logger *slog.Logger) *NodeRepository {
	return &NodeRepository{db: db, logger: logger}
}

func (r *NodeRepository) toDomain(m nodeModel) domain.Node {
	return domain.Node{ID: m.ID, URL: m.URL, GroupID: derefString(m.GroupID), Country: m.Country}
}

// IterateNodes streams nodes from storage using Go iterators.
func (r *NodeRepository) IterateNodes(ctx context.Context) iter.Seq2[domain.Node, error] {
	return func(yield func(domain.Node, error) bool) {
		models := make([]nodeModel, 0, 256)
		if err := r.db.WithContext(ctx).
			Select("id", "url", "group_id", "country").
			Find(&models).Error; err != nil {
			yield(domain.Node{}, fmt.Errorf("querying nodes: %w", err))
			return
		}
		for _, m := range models {
			if !yield(r.toDomain(m), nil) {
				return
			}
		}
	}
}

// ListVLESSURLs returns node URLs for subscription output, filtered by group if specified.
func (r *NodeRepository) ListVLESSURLs(ctx context.Context, groupID string, randomEnabled bool, randomLimit *int) ([]string, error) {
	type row struct {
		URL string `gorm:"column:url"`
	}

	query := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Select("url").
		Where("url LIKE ?", "vless://%")

	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	if randomEnabled {
		query = query.Order("RANDOM()")
	} else {
		query = query.Order("id ASC")
	}

	limit := 50
	if randomLimit != nil && *randomLimit > 0 {
		limit = *randomLimit
	}
	query = query.Limit(limit)

	rows := make([]row, 0, 64)
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("querying vless urls: %w", err)
	}

	urls := make([]string, 0, len(rows))
	for _, item := range rows {
		urls = append(urls, item.URL)
	}
	return urls, nil
}

// Create inserts a new node into the database.
func (r *NodeRepository) Create(ctx context.Context, node domain.Node) error {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		GroupID:   nullableString(node.GroupID),
		Country:   node.Country,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("creating node: %w", domain.ErrDuplicateNode)
		}
		return fmt.Errorf("creating node: %w", err)
	}
	r.logger.Debug("node created", slog.String("node_id", node.ID))
	return nil
}

// CreateIfAbsent inserts a node only when it does not already exist.
func (r *NodeRepository) CreateIfAbsent(ctx context.Context, node domain.Node) (bool, error) {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		GroupID:   nullableString(node.GroupID),
		Country:   node.Country,
		CreatedAt: time.Now().UTC(),
	}
	tx := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).
		Create(&model)
	if tx.Error != nil {
		return false, fmt.Errorf("creating node if absent: %w", tx.Error)
	}
	return tx.RowsAffected > 0, nil
}

// BulkCreateIfAbsent inserts multiple nodes; conflicts on id are ignored.
// Returns node IDs that were newly inserted.
func (r *NodeRepository) BulkCreateIfAbsent(ctx context.Context, nodes []domain.Node) ([]string, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(nodes))
	for i := range nodes {
		ids = append(ids, nodes[i].ID)
	}

	// Determine which IDs already exist to compute the inserted set.
	existing := make([]string, 0, len(ids))
	if err := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Where("id IN ?", ids).
		Pluck("id", &existing).Error; err != nil {
		return nil, fmt.Errorf("checking existing nodes: %w", err)
	}
	existingSet := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		existingSet[id] = struct{}{}
	}

	now := time.Now().UTC()
	models := make([]nodeModel, 0, len(nodes))
	inserted := make([]string, 0, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		if _, ok := existingSet[n.ID]; ok {
			continue
		}
		models = append(models, nodeModel{
			ID:        n.ID,
			URL:       n.URL,
			GroupID:   nullableString(n.GroupID),
			Country:   n.Country,
			CreatedAt: now,
		})
		inserted = append(inserted, n.ID)
	}

	if len(models) == 0 {
		return nil, nil
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).
		CreateInBatches(&models, 200).Error; err != nil {
		return nil, fmt.Errorf("bulk creating nodes: %w", err)
	}

	r.logger.Debug("nodes bulk-created", slog.Int("count", len(inserted)))
	return inserted, nil
}

// Upsert inserts a new node or updates url and group_id if it already exists.
func (r *NodeRepository) Upsert(ctx context.Context, node domain.Node) error {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		GroupID:   nullableString(node.GroupID),
		Country:   node.Country,
		CreatedAt: time.Now().UTC(),
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"url", "group_id"}),
		}).
		Create(&model).Error
	if err != nil {
		return fmt.Errorf("upserting node: %w", err)
	}
	r.logger.Debug("node upserted", slog.String("node_id", node.ID))
	return nil
}

// FindByID retrieves a node by ID.
func (r *NodeRepository) FindByID(ctx context.Context, id string) (domain.Node, error) {
	var model nodeModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Node{}, fmt.Errorf("node not found: %w", domain.ErrNodeNotFound)
		}
		return domain.Node{}, fmt.Errorf("finding node by id: %w", err)
	}
	return r.toDomain(model), nil
}

// List returns all nodes.
func (r *NodeRepository) List(ctx context.Context) ([]domain.Node, error) {
	var models []nodeModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	nodes := make([]domain.Node, 0, len(models))
	for _, m := range models {
		nodes = append(nodes, r.toDomain(m))
	}
	return nodes, nil
}

// ListPage returns paginated nodes with backend-level sorting.
func (r *NodeRepository) ListPage(ctx context.Context, limit int, offset int) ([]domain.Node, error) {
	var models []nodeModel
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing paged nodes: %w", err)
	}
	nodes := make([]domain.Node, 0, len(models))
	for _, m := range models {
		nodes = append(nodes, r.toDomain(m))
	}
	return nodes, nil
}

// ListPageByGroup returns paginated nodes for a single group.
func (r *NodeRepository) ListPageByGroup(ctx context.Context, groupID string, limit int, offset int) ([]domain.Node, error) {
	var models []nodeModel
	if err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing paged nodes by group: %w", err)
	}
	nodes := make([]domain.Node, 0, len(models))
	for _, m := range models {
		nodes = append(nodes, r.toDomain(m))
	}
	return nodes, nil
}

// ListByGroup returns all nodes in a group.
func (r *NodeRepository) ListByGroup(ctx context.Context, groupID string) ([]domain.Node, error) {
	var models []nodeModel
	if err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing nodes by group: %w", err)
	}
	nodes := make([]domain.Node, 0, len(models))
	for _, m := range models {
		nodes = append(nodes, r.toDomain(m))
	}
	return nodes, nil
}

// Update updates a node's URL or group.
func (r *NodeRepository) Update(ctx context.Context, node domain.Node) error {
	result := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Where("id = ?", node.ID).
		Updates(map[string]any{"url": node.URL, "group_id": nullableString(node.GroupID)})
	if result.Error != nil {
		return fmt.Errorf("updating node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("node not found: %w", domain.ErrNodeNotFound)
	}
	r.logger.Debug("node updated", slog.String("node_id", node.ID))
	return nil
}

// Delete removes a node by ID.
func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&nodeModel{})
	if result.Error != nil {
		return fmt.Errorf("deleting node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("node not found: %w", domain.ErrNodeNotFound)
	}
	r.logger.Debug("node deleted", slog.String("node_id", id))
	return nil
}
