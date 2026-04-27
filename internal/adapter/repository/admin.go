package repository

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

// AdminRepository persists admin users using GORM.
type AdminRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewAdminRepository constructs a GORM-backed admin repository.
func NewAdminRepository(db *gorm.DB, logger *slog.Logger) *AdminRepository {
	return &AdminRepository{db: db, logger: logger}
}

// FindByUsername retrieves an admin by username.
func (r *AdminRepository) FindByUsername(ctx context.Context, username string) (domain.Admin, error) {
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
func (r *AdminRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&adminModel{}).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("counting admins: %w", err)
	}

	return count, nil
}

// Create persists a new admin during first bootstrap.
func (r *AdminRepository) Create(ctx context.Context, admin domain.Admin) error {
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

// List returns all admins.
func (r *AdminRepository) List(ctx context.Context) ([]domain.Admin, error) {
	var models []adminModel
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing admins via gorm: %w", err)
	}

	admins := make([]domain.Admin, 0, len(models))
	for _, model := range models {
		admins = append(admins, domain.Admin{
			ID:           model.ID,
			Username:     model.Username,
			PasswordHash: model.PasswordHash,
		})
	}

	return admins, nil
}

// Update updates an admin's username and/or password.
// Only non-empty fields are applied.
func (r *AdminRepository) Update(ctx context.Context, admin domain.Admin) error {
	updates := make(map[string]any, 2)
	if admin.PasswordHash != "" {
		updates["password_hash"] = admin.PasswordHash
	}
	if admin.Username != "" {
		updates["username"] = admin.Username
	}

	if len(updates) == 0 {
		return nil
	}

	result := r.db.WithContext(ctx).
		Model(&adminModel{}).
		Where("id = ?", admin.ID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("updating admin via gorm: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("admin not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Debug("admin updated", slog.String("id", admin.ID))
	return nil
}

// Delete removes an admin by ID.
func (r *AdminRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&adminModel{})

	if result.Error != nil {
		return fmt.Errorf("deleting admin via gorm: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("admin not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Debug("admin deleted", slog.String("id", id))
	return nil
}
