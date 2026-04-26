package postgres

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"time"

	"outless/internal/domain"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type nodeModel struct {
	ID        string    `gorm:"column:id;primaryKey"`
	URL       string    `gorm:"column:url"`
	GroupID   *string   `gorm:"column:group_id"`
	Country   string    `gorm:"column:country"`
	CreatedAt time.Time `gorm:"column:created_at"`
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
			Select("id", "url", "group_id", "country").
			Find(&models).Error
		if err != nil {
			yield(domain.Node{}, fmt.Errorf("querying nodes via gorm: %w", err))
			return
		}

		for _, model := range models {
			groupID := ""
			if model.GroupID != nil {
				groupID = *model.GroupID
			}
			node := domain.Node{
				ID:      model.ID,
				URL:     model.URL,
				GroupID: groupID,
				Country: model.Country,
			}

			if !yield(node, nil) {
				return
			}
		}
	}
}

// ListVLESSURLs returns node URLs for subscription output, filtered by group if specified.
// Supports random selection and limit per group to handle large groups efficiently.
func (r *GormNodeRepository) ListVLESSURLs(ctx context.Context, groupID string, randomEnabled bool, randomLimit *int) ([]string, error) {
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

// Create inserts a new node into the database.
func (r *GormNodeRepository) Create(ctx context.Context, node domain.Node) error {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		GroupID:   nullableString(node.GroupID),
		Country:   node.Country,
		CreatedAt: time.Now().UTC(),
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("creating node via gorm: %w", domain.ErrDuplicateNode)
		}
		return fmt.Errorf("creating node via gorm: %w", err)
	}

	r.logger.Debug("node created", slog.String("node_id", node.ID))
	return nil
}

// CreateIfAbsent inserts a node only when it does not already exist.
// Returns true when a row was inserted.
func (r *GormNodeRepository) CreateIfAbsent(ctx context.Context, node domain.Node) (bool, error) {
	model := nodeModel{
		ID:        node.ID,
		URL:       node.URL,
		GroupID:   nullableString(node.GroupID),
		Country:   node.Country,
		CreatedAt: time.Now().UTC(),
	}

	tx := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).
		Create(&model)
	if tx.Error != nil {
		return false, fmt.Errorf("creating node if absent via gorm: %w", tx.Error)
	}

	created := tx.RowsAffected > 0
	if created {
		r.logger.Debug("node created", slog.String("node_id", node.ID))
	}
	return created, nil
}

// BulkCreateIfAbsent inserts multiple nodes in one round-trip; conflicts on id are ignored.
// Returns node IDs that were newly inserted.
func (r *GormNodeRepository) BulkCreateIfAbsent(ctx context.Context, nodes []domain.Node) ([]string, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	var b strings.Builder
	b.WriteString(`
INSERT INTO nodes (id, url, group_id, country, created_at)
VALUES `)
	args := make([]any, 0, len(nodes)*5)
	arg := 1
	for i := range nodes {
		n := &nodes[i]
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "($%d,$%d,$%d,$%d,$%d)",
			arg, arg+1, arg+2, arg+3, arg+4)
		arg += 5
		modelGroup := nullableString(n.GroupID)
		args = append(args,
			n.ID,
			n.URL,
			modelGroup,
			n.Country,
			now,
		)
	}
	b.WriteString(`
ON CONFLICT (id) DO NOTHING
RETURNING id`)

	rows, err := r.db.WithContext(ctx).Raw(b.String(), args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("bulk creating nodes if absent: %w", err)
	}
	defer rows.Close()

	inserted := make([]string, 0, len(nodes))
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("scanning inserted node id: %w", scanErr)
		}
		inserted = append(inserted, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating bulk insert result: %w", err)
	}

	if len(inserted) > 0 {
		r.logger.Debug("nodes bulk-created", slog.Int("count", len(inserted)))
	}
	return inserted, nil
}

// Upsert inserts a new node or updates url and group_id if the node already exists.
// This is atomic and safe for concurrent syncs.
func (r *GormNodeRepository) Upsert(ctx context.Context, node domain.Node) error {
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
		return fmt.Errorf("upserting node via gorm: %w", err)
	}

	r.logger.Debug("node upserted", slog.String("node_id", node.ID))
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
		GroupID: derefString(model.GroupID),
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
			GroupID: derefString(model.GroupID),
			Country: model.Country,
		})
	}

	return nodes, nil
}

// ListPage returns paginated nodes with backend-level sorting.
func (r *GormNodeRepository) ListPage(ctx context.Context, limit int, offset int) ([]domain.Node, error) {
	var models []nodeModel
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing paged nodes via gorm: %w", err)
	}

	nodes := make([]domain.Node, 0, len(models))
	for _, model := range models {
		nodes = append(nodes, domain.Node{
			ID:      model.ID,
			URL:     model.URL,
			GroupID: derefString(model.GroupID),
			Country: model.Country,
		})
	}

	return nodes, nil
}

// ListPageByGroup returns paginated nodes for a single group (same sort as ListPage).
func (r *GormNodeRepository) ListPageByGroup(ctx context.Context, groupID string, limit int, offset int) ([]domain.Node, error) {
	var models []nodeModel
	err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing paged nodes by group via gorm: %w", err)
	}

	nodes := make([]domain.Node, 0, len(models))
	for _, model := range models {
		nodes = append(nodes, domain.Node{
			ID:      model.ID,
			URL:     model.URL,
			GroupID: derefString(model.GroupID),
			Country: model.Country,
		})
	}

	return nodes, nil
}

// ListByGroup returns all nodes in a group.
func (r *GormNodeRepository) ListByGroup(ctx context.Context, groupID string) ([]domain.Node, error) {
	var models []nodeModel
	err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing nodes by group: %w", err)
	}

	nodes := make([]domain.Node, 0, len(models))
	for _, model := range models {
		nodes = append(nodes, domain.Node{
			ID:      model.ID,
			URL:     model.URL,
			GroupID: derefString(model.GroupID),
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
			"group_id": nullableString(node.GroupID),
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

func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
