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

type publicSourceModel struct {
	ID            string     `gorm:"column:id;primaryKey"`
	URL           string     `gorm:"column:url"`
	GroupID       string     `gorm:"column:group_id"`
	LastFetchedAt *time.Time `gorm:"column:last_fetched_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
}

func (publicSourceModel) TableName() string {
	return "public_sources"
}

// GormPublicSourceRepository persists public sources using GORM.
type GormPublicSourceRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGormPublicSourceRepository constructs a GORM-backed public source repository.
func NewGormPublicSourceRepository(db *gorm.DB, logger *slog.Logger) *GormPublicSourceRepository {
	return &GormPublicSourceRepository{db: db, logger: logger}
}

func (r *GormPublicSourceRepository) Create(ctx context.Context, source domain.PublicSource) error {
	model := publicSourceModel{
		ID:            source.ID,
		URL:           source.URL,
		GroupID:       source.GroupID,
		LastFetchedAt: source.LastFetchedAt,
		CreatedAt:     source.CreatedAt,
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("creating public source: %w", err)
	}

	r.logger.Info("public source created", slog.String("id", model.ID), slog.String("url", model.URL))
	return nil
}

func (r *GormPublicSourceRepository) FindByID(ctx context.Context, id string) (domain.PublicSource, error) {
	var model publicSourceModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.PublicSource{}, fmt.Errorf("public source not found: %w", domain.ErrNodeNotFound)
		}
		return domain.PublicSource{}, fmt.Errorf("finding public source by id: %w", err)
	}

	return domain.PublicSource{
		ID:            model.ID,
		URL:           model.URL,
		GroupID:       model.GroupID,
		LastFetchedAt: model.LastFetchedAt,
		CreatedAt:     model.CreatedAt,
	}, nil
}

func (r *GormPublicSourceRepository) List(ctx context.Context) ([]domain.PublicSource, error) {
	var models []publicSourceModel
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing public sources: %w", err)
	}

	sources := make([]domain.PublicSource, 0, len(models))
	for _, model := range models {
		sources = append(sources, domain.PublicSource{
			ID:            model.ID,
			URL:           model.URL,
			GroupID:       model.GroupID,
			LastFetchedAt: model.LastFetchedAt,
			CreatedAt:     model.CreatedAt,
		})
	}

	return sources, nil
}

func (r *GormPublicSourceRepository) Update(ctx context.Context, source domain.PublicSource) error {
	result := r.db.WithContext(ctx).
		Model(&publicSourceModel{}).
		Where("id = ?", source.ID).
		Updates(map[string]any{
			"url":             source.URL,
			"group_id":        source.GroupID,
			"last_fetched_at": source.LastFetchedAt,
		})

	if result.Error != nil {
		return fmt.Errorf("updating public source: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("public source not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Info("public source updated", slog.String("id", source.ID))
	return nil
}

func (r *GormPublicSourceRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&publicSourceModel{})

	if result.Error != nil {
		return fmt.Errorf("deleting public source: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("public source not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Info("public source deleted", slog.String("id", id))
	return nil
}

// GeneratePublicSourceID creates a unique public source ID.
func GeneratePublicSourceID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating public source id: %w", err)
	}
	return fmt.Sprintf("pubsrc_%d_%x", time.Now().UTC().Unix(), buf), nil
}
