package postgres

import (
	"context"
	"fmt"
	"log/slog"

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
