package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
)

type tokenModel struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Owner     string    `gorm:"column:owner"`
	TokenHash string    `gorm:"column:token_hash"`
	IsActive  bool      `gorm:"column:is_active"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (tokenModel) TableName() string {
	return "tokens"
}

// GormTokenRepository persists subscription tokens in PostgreSQL.
type GormTokenRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGormTokenRepository constructs a GORM-backed token repository.
func NewGormTokenRepository(db *gorm.DB, logger *slog.Logger) *GormTokenRepository {
	return &GormTokenRepository{db: db, logger: logger}
}

// IssueToken creates a new token and returns plain token only once.
func (r *GormTokenRepository) IssueToken(ctx context.Context, owner string, expiresAt time.Time) (string, error) {
	if strings.TrimSpace(owner) == "" {
		return "", fmt.Errorf("owner is required")
	}
	if expiresAt.IsZero() {
		return "", fmt.Errorf("expiresAt is required")
	}

	plainToken, err := generateToken(32)
	if err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}

	now := time.Now().UTC()
	tokenID, err := generateID(now)
	if err != nil {
		return "", fmt.Errorf("generating token id: %w", err)
	}

	model := tokenModel{
		ID:        tokenID,
		Owner:     owner,
		TokenHash: tokenHash(plainToken),
		IsActive:  true,
		ExpiresAt: expiresAt.UTC(),
		CreatedAt: now,
	}

	if err = r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return "", fmt.Errorf("creating token row: %w", err)
	}

	r.logger.Info("subscription token issued", slog.String("token_id", model.ID), slog.String("owner", owner), slog.Time("expires_at", model.ExpiresAt))
	return plainToken, nil
}

// ValidateToken verifies token activity and expiration.
func (r *GormTokenRepository) ValidateToken(ctx context.Context, token string, at time.Time) (bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).
		Model(&tokenModel{}).
		Where("token_hash = ? AND is_active = ? AND expires_at > ?", tokenHash(token), true, at.UTC()).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("validating token: %w", err)
	}

	return count > 0, nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateID(now time.Time) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return fmt.Sprintf("tok_%d_%x", now.Unix(), buf), nil
}
