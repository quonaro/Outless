package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"outless/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
)

type tokenModel struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Owner     string    `gorm:"column:owner"`
	GroupID   string    `gorm:"column:group_id"`
	TokenHash string    `gorm:"column:token_hash"`
	UUID      string    `gorm:"column:uuid"`
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
func (r *GormTokenRepository) IssueToken(ctx context.Context, owner string, groupID string, expiresAt time.Time) (string, error) {
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

	tokenUUID, err := generateUUIDv4()
	if err != nil {
		return "", fmt.Errorf("generating token uuid: %w", err)
	}

	model := tokenModel{
		ID:        tokenID,
		Owner:     owner,
		GroupID:   groupID,
		TokenHash: tokenHash(plainToken),
		UUID:      tokenUUID,
		IsActive:  true,
		ExpiresAt: expiresAt.UTC(),
		CreatedAt: now,
	}

	if err = r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return "", fmt.Errorf("creating token row: %w", err)
	}

	r.logger.Info("subscription token issued", slog.String("token_id", model.ID), slog.String("owner", owner), slog.String("group_id", groupID), slog.Time("expires_at", model.ExpiresAt))
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

// GetTokenGroupID returns the group ID associated with a token.
func (r *GormTokenRepository) GetTokenGroupID(ctx context.Context, token string, at time.Time) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", nil
	}

	var model tokenModel
	err := r.db.WithContext(ctx).
		Model(&tokenModel{}).
		Select("group_id").
		Where("token_hash = ? AND is_active = ? AND expires_at > ?", tokenHash(token), true, at.UTC()).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", fmt.Errorf("getting token group id: %w", err)
	}

	return model.GroupID, nil
}

// List returns all tokens.
func (r *GormTokenRepository) List(ctx context.Context) ([]domain.Token, error) {
	var models []tokenModel
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing tokens: %w", err)
	}

	tokens := make([]domain.Token, 0, len(models))
	for _, model := range models {
		tokens = append(tokens, toDomainToken(model))
	}

	return tokens, nil
}

// ListActive returns only active, non-expired tokens.
func (r *GormTokenRepository) ListActive(ctx context.Context, at time.Time) ([]domain.Token, error) {
	var models []tokenModel
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND expires_at > ?", true, at.UTC()).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing active tokens: %w", err)
	}

	tokens := make([]domain.Token, 0, len(models))
	for _, model := range models {
		tokens = append(tokens, toDomainToken(model))
	}

	return tokens, nil
}

// GetTokenByPlain returns token metadata (including UUID) for a valid plain token.
func (r *GormTokenRepository) GetTokenByPlain(ctx context.Context, token string, at time.Time) (domain.Token, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.Token{}, domain.ErrUnauthorized
	}

	var model tokenModel
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND is_active = ? AND expires_at > ?", tokenHash(token), true, at.UTC()).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Token{}, domain.ErrUnauthorized
		}
		return domain.Token{}, fmt.Errorf("fetching token by plain value: %w", err)
	}

	return toDomainToken(model), nil
}

func toDomainToken(model tokenModel) domain.Token {
	return domain.Token{
		ID:        model.ID,
		Owner:     model.Owner,
		GroupID:   model.GroupID,
		UUID:      model.UUID,
		IsActive:  model.IsActive,
		ExpiresAt: model.ExpiresAt,
		CreatedAt: model.CreatedAt,
	}
}

// Deactivates a token by ID.
func (r *GormTokenRepository) Deactivate(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Model(&tokenModel{}).
		Where("id = ?", id).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("deactivating token: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("token not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Info("token deactivated", slog.String("id", id))
	return nil
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

// generateUUIDv4 returns a random RFC 4122 v4 UUID string.
func generateUUIDv4() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}
