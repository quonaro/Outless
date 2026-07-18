package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"
	"outless/shared/crypto"

	"gorm.io/gorm"
)

type adminModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	Username     string `gorm:"column:username;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash"`
	TOTPSecret   string `gorm:"column:totp_secret"`
	TOTPEnabled  bool   `gorm:"column:totp_enabled"`
	CreatedAt    string `gorm:"column:created_at"`
}

func (adminModel) TableName() string { return "admins" }

// AdminRepository persists admin users using GORM over SQLite.
type AdminRepository struct {
	db        *gorm.DB
	logger    *slog.Logger
	cryptoKey [32]byte
}

// NewAdminRepository constructs a GORM-backed admin repository.
// The cryptoKey is used to encrypt TOTP secrets at rest.
func NewAdminRepository(db *gorm.DB, logger *slog.Logger, jwtSecret string) *AdminRepository {
	return &AdminRepository{
		db:        db,
		logger:    logger,
		cryptoKey: crypto.DeriveKey(jwtSecret),
	}
}

// FindByUsername retrieves an admin by username.
func (r *AdminRepository) FindByUsername(ctx context.Context, username string) (domain.Admin, error) {
	var model adminModel
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Admin{}, fmt.Errorf("admin not found: %w", domain.ErrAdminNotFound)
		}
		return domain.Admin{}, fmt.Errorf("querying admin by username: %w", err)
	}
	secret := model.TOTPSecret
	if secret != "" && crypto.IsEncrypted(secret) {
		decrypted, err := crypto.Decrypt(r.cryptoKey, secret)
		if err != nil {
			r.logger.Warn("failed to decrypt totp secret", slog.String("admin_id", model.ID), slog.String("error", err.Error()))
		} else {
			secret = decrypted
		}
	}
	return domain.Admin{
		ID:           model.ID,
		Username:     model.Username,
		PasswordHash: model.PasswordHash,
		TOTPSecret:   secret,
		TOTPEnabled:  model.TOTPEnabled,
	}, nil
}

// Count returns total admins in storage.
func (r *AdminRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&adminModel{}).Count(&count).Error; err != nil {
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
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing admins: %w", err)
	}
	admins := make([]domain.Admin, 0, len(models))
	for _, model := range models {
		secret := model.TOTPSecret
		if secret != "" && crypto.IsEncrypted(secret) {
			decrypted, err := crypto.Decrypt(r.cryptoKey, secret)
			if err != nil {
				r.logger.Warn("failed to decrypt totp secret", slog.String("admin_id", model.ID), slog.String("error", err.Error()))
			} else {
				secret = decrypted
			}
		}
		admins = append(admins, domain.Admin{
			ID:           model.ID,
			Username:     model.Username,
			PasswordHash: model.PasswordHash,
			TOTPSecret:   secret,
			TOTPEnabled:  model.TOTPEnabled,
		})
	}
	return admins, nil
}

// Update updates an admin's fields. Only non-empty / explicitly set fields apply.
func (r *AdminRepository) Update(ctx context.Context, admin domain.Admin) error {
	var existing adminModel
	if err := r.db.WithContext(ctx).Where("id = ?", admin.ID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("admin not found: %w", domain.ErrAdminNotFound)
		}
		return fmt.Errorf("updating admin: %w", err)
	}

	updates := make(map[string]any, 0)
	if admin.PasswordHash != "" {
		updates["password_hash"] = admin.PasswordHash
	}
	if admin.Username != "" {
		updates["username"] = admin.Username
	}
	if admin.TOTPSecret != "" {
		encrypted, err := crypto.Encrypt(r.cryptoKey, admin.TOTPSecret)
		if err != nil {
			return fmt.Errorf("encrypting totp secret: %w", err)
		}
		updates["totp_secret"] = encrypted
	}
	updates["totp_enabled"] = admin.TOTPEnabled
	if len(updates) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&adminModel{}).Where("id = ?", admin.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("updating admin: %w", err)
	}
	r.logger.Debug("admin updated", slog.String("id", admin.ID))
	return nil
}

// Delete removes an admin by ID.
func (r *AdminRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&adminModel{})
	if result.Error != nil {
		return fmt.Errorf("deleting admin: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("admin not found: %w", domain.ErrAdminNotFound)
	}
	r.logger.Debug("admin deleted", slog.String("id", id))
	return nil
}

// MigrateTOTPSecrets encrypts any plaintext TOTP secrets found in the database.
// Already-encrypted secrets are skipped. This is safe to run on every startup.
func (r *AdminRepository) MigrateTOTPSecrets(ctx context.Context) error {
	var models []adminModel
	if err := r.db.WithContext(ctx).Where("totp_secret != ''").Find(&models).Error; err != nil {
		return fmt.Errorf("querying admins with totp secrets: %w", err)
	}

	migrated := 0
	for _, m := range models {
		if crypto.IsEncrypted(m.TOTPSecret) {
			continue
		}

		encrypted, err := crypto.Encrypt(r.cryptoKey, m.TOTPSecret)
		if err != nil {
			r.logger.Error("failed to encrypt totp secret during migration",
				slog.String("admin_id", m.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		if err := r.db.WithContext(ctx).Model(&adminModel{}).
			Where("id = ?", m.ID).
			Update("totp_secret", encrypted).Error; err != nil {
			r.logger.Error("failed to update encrypted totp secret",
				slog.String("admin_id", m.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		migrated++
	}

	if migrated > 0 {
		r.logger.Info("totp secrets migrated to encrypted form", slog.Int("count", migrated))
	}
	return nil
}
