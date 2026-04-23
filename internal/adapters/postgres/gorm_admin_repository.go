package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type adminModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	Username     string `gorm:"column:username;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash"`
	CreatedAt    string `gorm:"column:created_at"`
}

func (adminModel) TableName() string {
	return "admins"
}

// GormAdminRepository persists admin users using GORM.
type GormAdminRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGormAdminRepository constructs a GORM-backed admin repository.
func NewGormAdminRepository(db *gorm.DB, logger *slog.Logger) *GormAdminRepository {
	return &GormAdminRepository{db: db, logger: logger}
}

// FindByUsername retrieves an admin by username.
func (r *GormAdminRepository) FindByUsername(ctx context.Context, username string) (domain.Admin, error) {
	var model adminModel
	err := r.db.WithContext(ctx).
		Where("username = ?", username).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Admin{}, fmt.Errorf("admin not found: %w", domain.ErrNodeNotFound)
		}
		return domain.Admin{}, fmt.Errorf("querying admin by username: %w", err)
	}

	return domain.Admin{
		ID:           model.ID,
		Username:     model.Username,
		PasswordHash: model.PasswordHash,
	}, nil
}

// Count returns total admins in storage.
func (r *GormAdminRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&adminModel{}).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("counting admins: %w", err)
	}

	return count, nil
}

// Create persists a new admin during first bootstrap.
func (r *GormAdminRepository) Create(ctx context.Context, admin domain.Admin) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&adminModel{}).Count(&count).Error; err != nil {
			return fmt.Errorf("counting admins in transaction: %w", err)
		}

		if count > 0 {
			return domain.ErrAdminAlreadyExists
		}

		model := adminModel{
			ID:           admin.ID,
			Username:     admin.Username,
			PasswordHash: admin.PasswordHash,
			CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}

		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("creating admin: %w", err)
		}

		return nil
	})
}
