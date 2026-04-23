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
	ID        string    `gorm:"column:id;primaryKey"`
	Name      string    `gorm:"column:name"`
	CreatedAt time.Time `gorm:"column:created_at"`
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
		ID:        group.ID,
		Name:      group.Name,
		CreatedAt: group.CreatedAt,
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
		ID:        model.ID,
		Name:      model.Name,
		CreatedAt: model.CreatedAt,
	}, nil
}

func (r *GormGroupRepository) List(ctx context.Context) ([]domain.Group, error) {
	var models []groupModel
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}

	groups := make([]domain.Group, 0, len(models))
	for _, model := range models {
		groups = append(groups, domain.Group{
			ID:        model.ID,
			Name:      model.Name,
			CreatedAt: model.CreatedAt,
		})
	}

	return groups, nil
}

func (r *GormGroupRepository) Update(ctx context.Context, group domain.Group) error {
	result := r.db.WithContext(ctx).
		Model(&groupModel{}).
		Where("id = ?", group.ID).
		Updates(map[string]any{
			"name": group.Name,
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
