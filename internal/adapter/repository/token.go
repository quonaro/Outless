package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"outless/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
)

type tokenModel struct {
	ID         string    `gorm:"column:id;primaryKey"`
	Owner      string    `gorm:"column:owner"`
	GroupID    *string   `gorm:"column:group_id"`
	TokenHash  string    `gorm:"column:token_hash"`
	TokenPlain *string   `gorm:"column:token_plain"`
	UUID       string    `gorm:"column:uuid"`
	IsActive   bool      `gorm:"column:is_active"`
	ExpiresAt  time.Time `gorm:"column:expires_at"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (tokenModel) TableName() string {
	return "tokens"
}

type tokenGroupModel struct {
	TokenID   string    `gorm:"column:token_id"`
	GroupID   string    `gorm:"column:group_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (tokenGroupModel) TableName() string {
	return "token_groups"
}

// TokenRepository persists subscription tokens in PostgreSQL.
type TokenRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewTokenRepository constructs a GORM-backed token repository.
func NewTokenRepository(db *gorm.DB, logger *slog.Logger) *TokenRepository {
	return &TokenRepository{db: db, logger: logger}
}

// IssueToken creates a new token and returns token metadata including plain token.
func (r *TokenRepository) IssueToken(ctx context.Context, owner string, groupIDs []string, expiresAt time.Time) (domain.Token, error) {
	if strings.TrimSpace(owner) == "" {
		return domain.Token{}, fmt.Errorf("owner is required")
	}
	if expiresAt.IsZero() {
		return domain.Token{}, fmt.Errorf("expiresAt is required")
	}

	plainToken, err := generateToken(32)
	if err != nil {
		return domain.Token{}, fmt.Errorf("generating token: %w", err)
	}

	now := time.Now().UTC()
	tokenID, err := generateID(now)
	if err != nil {
		return domain.Token{}, fmt.Errorf("generating token id: %w", err)
	}

	tokenUUID, err := generateUUIDv4()
	if err != nil {
		return domain.Token{}, fmt.Errorf("generating token uuid: %w", err)
	}

	legacyGroupID := ""
	if len(groupIDs) == 1 {
		legacyGroupID = groupIDs[0]
	}

	model := tokenModel{
		ID:         tokenID,
		Owner:      owner,
		GroupID:    nullableString(legacyGroupID),
		TokenHash:  tokenHash(plainToken),
		TokenPlain: nullableString(plainToken),
		UUID:       tokenUUID,
		IsActive:   true,
		ExpiresAt:  expiresAt.UTC(),
		CreatedAt:  now,
	}

	txErr := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("creating token row: %w", err)
		}

		for _, groupID := range uniqueNonEmpty(groupIDs) {
			link := tokenGroupModel{
				TokenID: tokenID,
				GroupID: groupID,
			}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("creating token_groups link: %w", err)
			}
		}

		return nil
	})
	if txErr != nil {
		return domain.Token{}, txErr
	}

	r.logger.Info("subscription token issued", slog.String("token_id", model.ID), slog.String("owner", owner), slog.Int("group_count", len(groupIDs)), slog.Time("expires_at", model.ExpiresAt))
	return toDomainToken(model, uniqueNonEmpty(groupIDs)), nil
}

// ValidateToken verifies token activity and expiration.
func (r *TokenRepository) ValidateToken(ctx context.Context, token string, at time.Time) (bool, error) {
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
func (r *TokenRepository) GetTokenGroupID(ctx context.Context, token string, at time.Time) (string, error) {
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

	groupIDs, err := r.loadGroupIDsByTokenIDs(ctx, []string{model.ID})
	if err != nil {
		return "", err
	}
	if groups := groupIDs[model.ID]; len(groups) > 0 {
		return groups[0], nil
	}
	return derefString(model.GroupID), nil
}

// List returns all tokens.
func (r *TokenRepository) List(ctx context.Context) ([]domain.Token, error) {
	var models []tokenModel
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing tokens: %w", err)
	}

	tokens := make([]domain.Token, 0, len(models))
	groupIDsMap, err := r.loadGroupIDsByTokenIDs(ctx, extractTokenIDs(models))
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		tokens = append(tokens, toDomainToken(model, groupIDsMap[model.ID]))
	}

	return tokens, nil
}

// ListActive returns only active, non-expired tokens.
func (r *TokenRepository) ListActive(ctx context.Context, at time.Time) ([]domain.Token, error) {
	var models []tokenModel
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND expires_at > ?", true, at.UTC()).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing active tokens: %w", err)
	}

	tokens := make([]domain.Token, 0, len(models))
	groupIDsMap, err := r.loadGroupIDsByTokenIDs(ctx, extractTokenIDs(models))
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		tokens = append(tokens, toDomainToken(model, groupIDsMap[model.ID]))
	}

	return tokens, nil
}

// GetTokenByPlain returns token metadata (including UUID) for a valid plain token.
func (r *TokenRepository) GetTokenByPlain(ctx context.Context, token string, at time.Time) (domain.Token, error) {
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

	groupIDsMap, err := r.loadGroupIDsByTokenIDs(ctx, []string{model.ID})
	if err != nil {
		return domain.Token{}, err
	}
	return toDomainToken(model, groupIDsMap[model.ID]), nil
}

func toDomainToken(model tokenModel, groupIDs []string) domain.Token {
	groupIDs = uniqueNonEmpty(groupIDs)
	legacyGroupID := derefString(model.GroupID)
	if len(groupIDs) == 0 && legacyGroupID != "" {
		groupIDs = []string{legacyGroupID}
	}
	primaryGroupID := ""
	if len(groupIDs) > 0 {
		primaryGroupID = groupIDs[0]
	}

	return domain.Token{
		ID:         model.ID,
		Owner:      model.Owner,
		GroupID:    primaryGroupID,
		GroupIDs:   groupIDs,
		TokenPlain: derefString(model.TokenPlain),
		UUID:       model.UUID,
		IsActive:   model.IsActive,
		ExpiresAt:  model.ExpiresAt,
		CreatedAt:  model.CreatedAt,
	}
}

// FindByID retrieves a token by its ID.
func (r *TokenRepository) FindByID(ctx context.Context, id string) (domain.Token, error) {
	var model tokenModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Token{}, fmt.Errorf("token not found: %w", domain.ErrNodeNotFound)
		}
		return domain.Token{}, fmt.Errorf("finding token: %w", err)
	}

	groupIDsMap, err := r.loadGroupIDsByTokenIDs(ctx, []string{model.ID})
	if err != nil {
		return domain.Token{}, err
	}
	return toDomainToken(model, groupIDsMap[model.ID]), nil
}

// Deactivates a token by ID.
func (r *TokenRepository) Deactivate(ctx context.Context, id string) error {
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

// Activate reactivates a token by ID.
func (r *TokenRepository) Activate(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Model(&tokenModel{}).
		Where("id = ?", id).
		Update("is_active", true)

	if result.Error != nil {
		return fmt.Errorf("activating token: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("token not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Info("token activated", slog.String("id", id))
	return nil
}

// Remove permanently deletes a token by ID.
func (r *TokenRepository) Remove(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&tokenModel{})

	if result.Error != nil {
		return fmt.Errorf("removing token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("token not found: %w", domain.ErrNodeNotFound)
	}

	r.logger.Info("token removed", slog.String("id", id))
	return nil
}

// Update modifies token owner, group IDs, and expiration.
func (r *TokenRepository) Update(ctx context.Context, id string, owner string, groupIDs []string, expiresAt time.Time) error {
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("owner is required")
	}
	if expiresAt.IsZero() {
		return fmt.Errorf("expiresAt is required")
	}

	legacyGroupID := ""
	if len(groupIDs) == 1 {
		legacyGroupID = groupIDs[0]
	}

	txErr := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update token row
		result := tx.Model(&tokenModel{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"owner":      owner,
				"group_id":   nullableString(legacyGroupID),
				"expires_at": expiresAt.UTC(),
			})

		if result.Error != nil {
			return fmt.Errorf("updating token: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("token not found: %w", domain.ErrNodeNotFound)
		}

		// Delete existing group links
		if err := tx.Where("token_id = ?", id).Delete(&tokenGroupModel{}).Error; err != nil {
			return fmt.Errorf("deleting old group links: %w", err)
		}

		// Insert new group links
		for _, groupID := range uniqueNonEmpty(groupIDs) {
			link := tokenGroupModel{
				TokenID: id,
				GroupID: groupID,
			}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("creating token_groups link: %w", err)
			}
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	r.logger.Info("token updated", slog.String("id", id), slog.String("owner", owner), slog.Int("group_count", len(groupIDs)), slog.Time("expires_at", expiresAt))
	return nil
}

func (r *TokenRepository) loadGroupIDsByTokenIDs(ctx context.Context, tokenIDs []string) (map[string][]string, error) {
	if len(tokenIDs) == 0 {
		return map[string][]string{}, nil
	}

	rows := make([]tokenGroupModel, 0, len(tokenIDs))
	if err := r.db.WithContext(ctx).
		Where("token_id IN ?", tokenIDs).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("loading token group links: %w", err)
	}

	out := make(map[string][]string, len(tokenIDs))
	for _, row := range rows {
		out[row.TokenID] = append(out[row.TokenID], row.GroupID)
	}
	return out, nil
}

func extractTokenIDs(models []tokenModel) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
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
