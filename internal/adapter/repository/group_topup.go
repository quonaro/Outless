package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type groupTopUpModel struct {
	ID           string     `gorm:"column:id;primaryKey"`
	GroupID      string     `gorm:"column:group_id;uniqueIndex"`
	URLs         string     `gorm:"column:urls;type:text"`
	ParserType   string     `gorm:"column:parser_type"`
	ParserParams string     `gorm:"column:parser_params;type:text"`
	CheckEnabled bool       `gorm:"column:check_enabled"`
	CheckConfig  string     `gorm:"column:check_config;type:text"`
	ScheduleType string     `gorm:"column:schedule_type"`
	ScheduleExpr string     `gorm:"column:schedule_expr"`
	NextRunAt    time.Time  `gorm:"column:next_run_at;index"`
	LastRunAt    *time.Time `gorm:"column:last_run_at"`
	Enabled      bool       `gorm:"column:enabled"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (groupTopUpModel) TableName() string { return "group_top_ups" }

// GroupTopUpRepository persists group top-up settings using GORM over SQLite.
type GroupTopUpRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGroupTopUpRepository constructs a GORM-backed group top-up repository.
func NewGroupTopUpRepository(db *gorm.DB, logger *slog.Logger) *GroupTopUpRepository {
	return &GroupTopUpRepository{db: db, logger: logger}
}

func (r *GroupTopUpRepository) toDomain(m groupTopUpModel) (domain.GroupTopUp, error) {
	urls, err := decodeStringSlice(m.URLs)
	if err != nil {
		return domain.GroupTopUp{}, fmt.Errorf("decoding top-up urls: %w", err)
	}

	parserParams, err := decodeMap(m.ParserParams)
	if err != nil {
		return domain.GroupTopUp{}, fmt.Errorf("decoding top-up parser params: %w", err)
	}

	checkConfig, err := decodeCheckConfig(m.CheckConfig)
	if err != nil {
		return domain.GroupTopUp{}, fmt.Errorf("decoding top-up check config: %w", err)
	}

	return domain.GroupTopUp{
		ID:           m.ID,
		GroupID:      m.GroupID,
		URLs:         urls,
		ParserType:   m.ParserType,
		ParserParams: parserParams,
		CheckEnabled: m.CheckEnabled,
		CheckConfig:  checkConfig,
		ScheduleType: m.ScheduleType,
		ScheduleExpr: m.ScheduleExpr,
		NextRunAt:    m.NextRunAt,
		LastRunAt:    m.LastRunAt,
		Enabled:      m.Enabled,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}, nil
}

func (r *GroupTopUpRepository) toModel(t domain.GroupTopUp) (groupTopUpModel, error) {
	urls, err := encodeStringSlice(t.URLs)
	if err != nil {
		return groupTopUpModel{}, fmt.Errorf("encoding top-up urls: %w", err)
	}

	parserParams, err := encodeMap(t.ParserParams)
	if err != nil {
		return groupTopUpModel{}, fmt.Errorf("encoding top-up parser params: %w", err)
	}

	checkConfig, err := encodeCheckConfig(t.CheckConfig)
	if err != nil {
		return groupTopUpModel{}, fmt.Errorf("encoding top-up check config: %w", err)
	}

	return groupTopUpModel{
		ID:           t.ID,
		GroupID:      t.GroupID,
		URLs:         urls,
		ParserType:   t.ParserType,
		ParserParams: parserParams,
		CheckEnabled: t.CheckEnabled,
		CheckConfig:  checkConfig,
		ScheduleType: t.ScheduleType,
		ScheduleExpr: t.ScheduleExpr,
		NextRunAt:    t.NextRunAt,
		LastRunAt:    t.LastRunAt,
		Enabled:      t.Enabled,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}, nil
}

// Create inserts a new group top-up record.
func (r *GroupTopUpRepository) Create(ctx context.Context, topUp domain.GroupTopUp) error {
	model, err := r.toModel(topUp)
	if err != nil {
		return err
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now().UTC()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = model.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("creating group top-up: %w", err)
	}
	r.logger.Info("group top-up created", slog.String("id", model.ID), slog.String(groupIDColumn, model.GroupID))
	return nil
}

// FindByID retrieves a group top-up by ID.
func (r *GroupTopUpRepository) FindByID(ctx context.Context, id string) (domain.GroupTopUp, error) {
	var model groupTopUpModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.GroupTopUp{}, fmt.Errorf("group top-up not found: %w", domain.ErrGroupTopUpNotFound)
		}
		return domain.GroupTopUp{}, fmt.Errorf("finding group top-up by id: %w", err)
	}
	return r.toDomain(model)
}

// FindByGroupID retrieves a group top-up by group ID.
func (r *GroupTopUpRepository) FindByGroupID(ctx context.Context, groupID string) (domain.GroupTopUp, error) {
	var model groupTopUpModel
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.GroupTopUp{}, fmt.Errorf("group top-up not found: %w", domain.ErrGroupTopUpNotFound)
		}
		return domain.GroupTopUp{}, fmt.Errorf("finding group top-up by group id: %w", err)
	}
	return r.toDomain(model)
}

// List returns all group top-up records.
func (r *GroupTopUpRepository) List(ctx context.Context) ([]domain.GroupTopUp, error) {
	var models []groupTopUpModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing group top-ups: %w", err)
	}

	topUps := make([]domain.GroupTopUp, 0, len(models))
	for _, m := range models {
		t, err := r.toDomain(m)
		if err != nil {
			return nil, err
		}
		topUps = append(topUps, t)
	}
	return topUps, nil
}

// ListDue returns enabled top-ups whose next run is at or before the given time.
func (r *GroupTopUpRepository) ListDue(ctx context.Context, at time.Time) ([]domain.GroupTopUp, error) {
	var models []groupTopUpModel
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND next_run_at <= ?", true, at.UTC()).
		Order("next_run_at ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing due group top-ups: %w", err)
	}

	topUps := make([]domain.GroupTopUp, 0, len(models))
	for _, m := range models {
		t, err := r.toDomain(m)
		if err != nil {
			return nil, err
		}
		topUps = append(topUps, t)
	}
	return topUps, nil
}

// Update updates a group top-up record.
func (r *GroupTopUpRepository) Update(ctx context.Context, topUp domain.GroupTopUp) error {
	model, err := r.toModel(topUp)
	if err != nil {
		return err
	}
	model.UpdatedAt = time.Now().UTC()

	result := r.db.WithContext(ctx).Model(&groupTopUpModel{}).Where("id = ?", model.ID).Updates(map[string]any{
		groupIDColumn:   model.GroupID,
		"urls":          model.URLs,
		"parser_type":   model.ParserType,
		"parser_params": model.ParserParams,
		"check_enabled": model.CheckEnabled,
		"check_config":  model.CheckConfig,
		"schedule_type": model.ScheduleType,
		"schedule_expr": model.ScheduleExpr,
		"next_run_at":   model.NextRunAt,
		"last_run_at":   model.LastRunAt,
		"enabled":       model.Enabled,
		"updated_at":    model.UpdatedAt,
	})
	if result.Error != nil {
		return fmt.Errorf("updating group top-up: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("group top-up not found: %w", domain.ErrGroupTopUpNotFound)
	}
	r.logger.Info("group top-up updated", slog.String("id", model.ID))
	return nil
}

// Delete removes a group top-up by ID.
func (r *GroupTopUpRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&groupTopUpModel{})
	if result.Error != nil {
		return fmt.Errorf("deleting group top-up: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("group top-up not found: %w", domain.ErrGroupTopUpNotFound)
	}
	r.logger.Info("group top-up deleted", slog.String("id", id))
	return nil
}

func encodeStringSlice(v []string) (string, error) {
	if v == nil {
		return "[]", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeStringSlice(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var v []string
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return v, nil
}

func encodeMap(v map[string]any) (string, error) {
	if v == nil {
		return "{}", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeMap(s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return v, nil
}

func encodeCheckConfig(v domain.TopUpCheckConfig) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeCheckConfig(s string) (domain.TopUpCheckConfig, error) {
	var v domain.TopUpCheckConfig
	if s == "" {
		return v, nil
	}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return v, err
	}
	return v, nil
}
