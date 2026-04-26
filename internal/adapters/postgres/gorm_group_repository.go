package postgres

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type groupModel struct {
	ID                    string     `gorm:"column:id;primaryKey"`
	Name                  string     `gorm:"column:name"`
	SourceURL             *string    `gorm:"column:source_url"`
	TotalNodes            int64      `gorm:"column:total_nodes"`
	HealthyNodes          int64      `gorm:"column:healthy_nodes"`
	UnhealthyNodes        int64      `gorm:"column:unhealthy_nodes"`
	UnknownNodes          int64      `gorm:"column:unknown_nodes"`
	AutoDeleteUnavailable bool       `gorm:"column:auto_delete_unavailable"`
	RandomEnabled         bool       `gorm:"column:random_enabled"`
	RandomLimit           *int64     `gorm:"column:random_limit"`
	LastSyncedAt          *time.Time `gorm:"column:last_synced_at"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
}

func (groupModel) TableName() string {
	return "groups"
}

// GormGroupRepository persists groups using GORM.
type GormGroupRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGormGroupRepository constructs a GORM-backed group repository.
func NewGormGroupRepository(db *gorm.DB, logger *slog.Logger) *GormGroupRepository {
	return &GormGroupRepository{db: db, logger: logger}
}

func (r *GormGroupRepository) Create(ctx context.Context, group domain.Group) error {
	model := groupModel{
		ID:            group.ID,
		Name:          group.Name,
		SourceURL:     nullableGroupString(group.SourceURL),
		RandomEnabled: group.RandomEnabled,
		RandomLimit:   nullableGroupInt(group.RandomLimit),
		LastSyncedAt:  group.LastSyncedAt,
		CreatedAt:     group.CreatedAt,
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("creating group: %w", err)
	}

	r.logger.Info("group created", slog.String("id", model.ID), slog.String("name", model.Name))
	return nil
}

func (r *GormGroupRepository) FindByID(ctx context.Context, id string) (domain.Group, error) {
	var model groupModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Group{}, fmt.Errorf("group not found: %w", domain.ErrNodeNotFound)
		}
		return domain.Group{}, fmt.Errorf("finding group by id: %w", err)
	}

	return domain.Group{
		ID:            model.ID,
		Name:          model.Name,
		SourceURL:     derefGroupString(model.SourceURL),
		RandomEnabled: model.RandomEnabled,
		RandomLimit:   derefGroupInt(model.RandomLimit),
		LastSyncedAt:  model.LastSyncedAt,
		CreatedAt:     model.CreatedAt,
	}, nil
}

func (r *GormGroupRepository) List(ctx context.Context) ([]domain.Group, error) {
	var models []groupModel
	err := r.db.WithContext(ctx).
		Model(&groupModel{}).
		Select(
			"groups.id", "groups.name", "groups.source_url", "groups.random_enabled", "groups.random_limit", "groups.last_synced_at", "groups.created_at",
			"COUNT(nodes.id) AS total_nodes",
		).
		Joins("LEFT JOIN nodes ON nodes.group_id = groups.id").
		Group("groups.id").
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}

	groups := make([]domain.Group, 0, len(models))
	for _, model := range models {
		groups = append(groups, domain.Group{
			ID:            model.ID,
			Name:          model.Name,
			SourceURL:     derefGroupString(model.SourceURL),
			TotalNodes:    int(model.TotalNodes),
			RandomEnabled: model.RandomEnabled,
			RandomLimit:   derefGroupInt(model.RandomLimit),
			LastSyncedAt:  model.LastSyncedAt,
			CreatedAt:     model.CreatedAt,
		})
	}

	return groups, nil
}

func (r *GormGroupRepository) Update(ctx context.Context, group domain.Group) error {
	result := r.db.WithContext(ctx).
		Model(&groupModel{}).
		Where("id = ?", group.ID).
		Updates(map[string]any{
			"name":           group.Name,
			"source_url":     nullableGroupString(group.SourceURL),
			"random_enabled": group.RandomEnabled,
			"random_limit":   nullableGroupInt(group.RandomLimit),
			"last_synced_at": group.LastSyncedAt,
		})

	if result.Error != nil {
		return fmt.Errorf("updating group: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Info("group updated", slog.String("id", group.ID))
	return nil
}

func (r *GormGroupRepository) UpdateSyncedAt(ctx context.Context, id string, syncedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&groupModel{}).
		Where("id = ?", id).
		Update("last_synced_at", syncedAt.UTC())
	if result.Error != nil {
		return fmt.Errorf("updating group sync timestamp: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found: %w", domain.ErrNodeNotFound)
	}
	return nil
}

func (r *GormGroupRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&groupModel{})

	if result.Error != nil {
		return fmt.Errorf("deleting group: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("group not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Info("group deleted", slog.String("id", id))
	return nil
}

// GenerateGroupID creates a unique group ID.
func GenerateGroupID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating group id: %w", err)
	}
	return fmt.Sprintf("grp_%d_%x", time.Now().UTC().Unix(), buf), nil
}

func nullableGroupString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func derefGroupString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
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
