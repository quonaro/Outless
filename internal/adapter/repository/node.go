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
	Country   string    `gorm:"column:country"`
	IsSelf    bool      `gorm:"column:is_self;index"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (nodeModel) TableName() string { return "nodes" }

type nodeGroupModel struct {
	NodeID  string `gorm:"column:node_id;primaryKey"`
	GroupID string `gorm:"column:group_id;primaryKey"`
}

func (nodeGroupModel) TableName() string { return "node_groups" }

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
	return domain.Node{ID: m.ID, URL: m.URL, Country: m.Country, IsSelf: m.IsSelf}
}

func (r *NodeRepository) fillGroupIDs(ctx context.Context, nodes []domain.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	nodeIDs := make([]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}
	type row struct {
		NodeID  string `gorm:"column:node_id"`
		GroupID string `gorm:"column:group_id"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&nodeGroupModel{}).
		Where("node_id IN ?", nodeIDs).
		Find(&rows).Error; err != nil {
		return fmt.Errorf("querying node groups: %w", err)
	}
	groupMap := make(map[string][]string, len(nodes))
	for _, item := range rows {
		groupMap[item.NodeID] = append(groupMap[item.NodeID], item.GroupID)
	}
	for i := range nodes {
		nodes[i].GroupIDs = groupMap[nodes[i].ID]
	}
	return nil
}

func (r *NodeRepository) createGroupLinks(ctx context.Context, nodeID string, groupIDs []string) error {
	if len(groupIDs) == 0 {
		return nil
	}
	models := make([]nodeGroupModel, 0, len(groupIDs))
	for _, gid := range groupIDs {
		models = append(models, nodeGroupModel{NodeID: nodeID, GroupID: gid})
	}
	if err := r.db.WithContext(ctx).Create(&models).Error; err != nil {
		return fmt.Errorf("creating node groups: %w", err)
	}
	return nil
}

func (r *NodeRepository) replaceGroupLinks(ctx context.Context, nodeID string, groupIDs []string) error {
	if err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).Delete(&nodeGroupModel{}).Error; err != nil {
		return fmt.Errorf("deleting old node groups: %w", err)
	}
	return r.createGroupLinks(ctx, nodeID, groupIDs)
}

// IterateNodes streams nodes from storage using Go iterators.
func (r *NodeRepository) IterateNodes(ctx context.Context) iter.Seq2[domain.Node, error] {
	return func(yield func(domain.Node, error) bool) {
		models := make([]nodeModel, 0, 256)
		if err := r.db.WithContext(ctx).
			Select("id", "url", "country", "is_self").
			Find(&models).Error; err != nil {
			yield(domain.Node{}, fmt.Errorf("querying nodes: %w", err))
			return
		}
		nodes := make([]domain.Node, 0, len(models))
		for _, m := range models {
			nodes = append(nodes, r.toDomain(m))
		}
		if err := r.fillGroupIDs(ctx, nodes); err != nil {
			yield(domain.Node{}, err)
			return
		}
		for _, n := range nodes {
			if !yield(n, nil) {
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
		Select("nodes.url").
		Where("nodes.url LIKE ?", "vless://%")

	if groupID != "" {
		query = query.Joins("JOIN node_groups ON node_groups.node_id = nodes.id").
			Where("node_groups.group_id = ?", groupID)
	}
	if randomEnabled {
		query = query.Order("RANDOM()")
	} else {
		query = query.Order("nodes.id ASC")
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
		Country:   node.Country,
		IsSelf:    node.IsSelf,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("creating node: %w", domain.ErrDuplicateNode)
		}
		return fmt.Errorf("creating node: %w", err)
	}
	if err := r.createGroupLinks(ctx, node.ID, node.GroupIDs); err != nil {
		return err
	}
	r.logger.Debug("node created", slog.String("node_id", node.ID))
	return nil
}

// CreateIfAbsent inserts a node only when it does not already exist.
func (r *NodeRepository) CreateIfAbsent(ctx context.Context, node domain.Node) (bool, error) {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		Country:   node.Country,
		IsSelf:    node.IsSelf,
		CreatedAt: time.Now().UTC(),
	}
	tx := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).
		Create(&model)
	if tx.Error != nil {
		return false, fmt.Errorf("creating node if absent: %w", tx.Error)
	}
	if tx.RowsAffected > 0 {
		if err := r.createGroupLinks(ctx, node.ID, node.GroupIDs); err != nil {
			return false, err
		}
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
			Country:   n.Country,
			IsSelf:    n.IsSelf,
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

	for _, n := range nodes {
		if _, ok := existingSet[n.ID]; ok {
			continue
		}
		if err := r.createGroupLinks(ctx, n.ID, n.GroupIDs); err != nil {
			r.logger.Warn("failed to create node groups", slog.String("node_id", n.ID), slog.String("error", err.Error()))
		}
	}

	r.logger.Debug("nodes bulk-created", slog.Int("count", len(inserted)))
	return inserted, nil
}

// Upsert inserts a new node or updates url if it already exists.
func (r *NodeRepository) Upsert(ctx context.Context, node domain.Node) error {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		Country:   node.Country,
		IsSelf:    node.IsSelf,
		CreatedAt: time.Now().UTC(),
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"url"}),
		}).
		Create(&model).Error
	if err != nil {
		return fmt.Errorf("upserting node: %w", err)
	}
	if err := r.replaceGroupLinks(ctx, node.ID, node.GroupIDs); err != nil {
		return err
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
	node := r.toDomain(model)
	if err := r.fillGroupIDs(ctx, []domain.Node{node}); err != nil {
		return domain.Node{}, err
	}
	return node, nil
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
	if err := r.fillGroupIDs(ctx, nodes); err != nil {
		return nil, err
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
	if err := r.fillGroupIDs(ctx, nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// ListPageByGroup returns paginated nodes for a single group.
func (r *NodeRepository) ListPageByGroup(ctx context.Context, groupID string, limit int, offset int) ([]domain.Node, error) {
	var models []nodeModel
	if err := r.db.WithContext(ctx).
		Joins("JOIN node_groups ON node_groups.node_id = nodes.id").
		Where("node_groups.group_id = ?", groupID).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing paged nodes by group: %w", err)
	}
	nodes := make([]domain.Node, 0, len(models))
	for _, m := range models {
		nodes = append(nodes, r.toDomain(m))
	}
	if err := r.fillGroupIDs(ctx, nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// ListByGroup returns all nodes in a group.
func (r *NodeRepository) ListByGroup(ctx context.Context, groupID string) ([]domain.Node, error) {
	var models []nodeModel
	if err := r.db.WithContext(ctx).
		Joins("JOIN node_groups ON node_groups.node_id = nodes.id").
		Where("node_groups.group_id = ?", groupID).
		Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing nodes by group: %w", err)
	}
	nodes := make([]domain.Node, 0, len(models))
	for _, m := range models {
		nodes = append(nodes, r.toDomain(m))
	}
	if err := r.fillGroupIDs(ctx, nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// Update updates a node's URL or group associations.
func (r *NodeRepository) Update(ctx context.Context, node domain.Node) error {
	result := r.db.WithContext(ctx).
		Model(&nodeModel{}).
		Where("id = ?", node.ID).
		Updates(map[string]any{"url": node.URL, "is_self": node.IsSelf})
	if result.Error != nil {
		return fmt.Errorf("updating node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("node not found: %w", domain.ErrNodeNotFound)
	}
	if err := r.replaceGroupLinks(ctx, node.ID, node.GroupIDs); err != nil {
		return err
	}
	r.logger.Debug("node updated", slog.String("node_id", node.ID))
	return nil
}

// HasSelfNode reports whether a self-node already exists.
func (r *NodeRepository) HasSelfNode(ctx context.Context) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&nodeModel{}).Where("is_self = ?", true).Count(&count).Error; err != nil {
		return false, fmt.Errorf("checking self node: %w", err)
	}
	return count > 0, nil
}

// Delete removes a node by ID.
func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("node_id = ?", id).Delete(&nodeGroupModel{}).Error; err != nil {
		return fmt.Errorf("deleting node groups: %w", err)
	}
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
