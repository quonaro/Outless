package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type groupModel struct {
	ID            string    `gorm:"column:id;primaryKey"`
	Name          string    `gorm:"column:name"`
	TotalNodes    int64     `gorm:"column:total_nodes"`
	RandomEnabled bool      `gorm:"column:random_enabled"`
	RandomLimit   *int64    `gorm:"column:random_limit"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (groupModel) TableName() string { return "groups" }

// GroupRepository persists groups using GORM over SQLite.
type GroupRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGroupRepository constructs a GORM-backed group repository.
func NewGroupRepository(db *gorm.DB, logger *slog.Logger) *GroupRepository {
	return &GroupRepository{db: db, logger: logger}
}

func (r *GroupRepository) Create(ctx context.Context, group domain.Group) error {
	model := groupModel{
		ID:            group.ID,
		Name:          group.Name,
		RandomEnabled: group.RandomEnabled,
		RandomLimit:   nullableGroupInt(group.RandomLimit),
		CreatedAt:     group.CreatedAt,
	}
	if !model.RandomEnabled && model.RandomLimit != nil {
		model.RandomEnabled = true
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("creating group: %w", err)
	}
	r.logger.Info("group created", slog.String("id", model.ID), slog.String("name", model.Name))
	return nil
}

func (r *GroupRepository) FindByID(ctx context.Context, id string) (domain.Group, error) {
	var model groupModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Group{}, fmt.Errorf("group not found: %w", domain.ErrGroupNotFound)
		}
		return domain.Group{}, fmt.Errorf("finding group by id: %w", err)
	}
	return domain.Group{
		ID:            model.ID,
		Name:          model.Name,
		RandomEnabled: model.RandomEnabled,
		RandomLimit:   derefGroupInt(model.RandomLimit),
		CreatedAt:     model.CreatedAt,
	}, nil
}

func (r *GroupRepository) List(ctx context.Context) ([]domain.Group, error) {
	var models []groupModel
	err := r.db.WithContext(ctx).
		Model(&groupModel{}).
		Select(
			"groups.id", "groups.name", "groups.random_enabled",
			"groups.random_limit", "groups.created_at",
			"COUNT(node_groups.node_id) AS total_nodes",
		).
		Joins("LEFT JOIN node_groups ON node_groups.group_id = groups.id").
		Group("groups.id").
		Order("groups.created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	groups := make([]domain.Group, 0, len(models))
	for _, model := range models {
		groups = append(groups, domain.Group{
			ID:            model.ID,
			Name:          model.Name,
			TotalNodes:    int(model.TotalNodes),
			RandomEnabled: model.RandomEnabled,
			RandomLimit:   derefGroupInt(model.RandomLimit),
			CreatedAt:     model.CreatedAt,
		})
	}
	return groups, nil
}

func (r *GroupRepository) Update(ctx context.Context, group domain.Group) error {
	updates := map[string]any{
		"name":           group.Name,
		"random_enabled": group.RandomEnabled,
		"random_limit":   nullableGroupInt(group.RandomLimit),
	}
	if !group.RandomEnabled && group.RandomLimit != nil {
		updates["random_enabled"] = true
	}
	result := r.db.WithContext(ctx).Model(&groupModel{}).Where("id = ?", group.ID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("updating group: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found: %w", domain.ErrGroupNotFound)
	}
	r.logger.Info("group updated", slog.String("id", group.ID))
	return nil
}

func (r *GroupRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&groupModel{})
	if result.Error != nil {
		return fmt.Errorf("deleting group: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found: %w", domain.ErrGroupNotFound)
	}
	r.logger.Info("group deleted", slog.String("id", id))
	return nil
}

func nullableGroupInt(v *int) *int64 {
	if v == nil {
		return nil
	}
	val := int64(*v)
	return &val
}

func derefGroupInt(v *int64) *int {
	if v == nil {
		return nil
	}
	val := int(*v)
	return &val
}
