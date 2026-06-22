package repository

import (
	"context"
	"fmt"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type tokenUsageModel struct {
	TokenID       string    `gorm:"column:token_id;primaryKey"`
	PeriodType    string    `gorm:"column:period_type;primaryKey"`
	PeriodStart   time.Time `gorm:"column:period_start;primaryKey"`
	UploadBytes   int64     `gorm:"column:upload_bytes"`
	DownloadBytes int64     `gorm:"column:download_bytes"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (tokenUsageModel) TableName() string { return "token_usage" }

type nodeUsageModel struct {
	NodeID        string    `gorm:"column:node_id;primaryKey"`
	PeriodType    string    `gorm:"column:period_type;primaryKey"`
	PeriodStart   time.Time `gorm:"column:period_start;primaryKey"`
	UploadBytes   int64     `gorm:"column:upload_bytes"`
	DownloadBytes int64     `gorm:"column:download_bytes"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (nodeUsageModel) TableName() string { return "node_usage" }

type inboundUsageModel struct {
	InboundTag    string    `gorm:"column:inbound_tag;primaryKey"`
	PeriodType    string    `gorm:"column:period_type;primaryKey"`
	PeriodStart   time.Time `gorm:"column:period_start;primaryKey"`
	UploadBytes   int64     `gorm:"column:upload_bytes"`
	DownloadBytes int64     `gorm:"column:download_bytes"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (inboundUsageModel) TableName() string { return "inbound_usage" }

// TrafficRepository persists per-token traffic usage aggregates in SQLite.
type TrafficRepository struct {
	db *gorm.DB
}

// NewTrafficRepository constructs a GORM-backed traffic repository.
func NewTrafficRepository(db *gorm.DB) *TrafficRepository {
	return &TrafficRepository{db: db}
}

// RecordUsage upserts traffic usage for a token and period.
func (r *TrafficRepository) RecordUsage(ctx context.Context, usage domain.TokenUsage) error {
	model := tokenUsageModel{
		TokenID:       usage.TokenID,
		PeriodType:    usage.PeriodType,
		PeriodStart:   usage.PeriodStart.UTC(),
		UploadBytes:   usage.UploadBytes,
		DownloadBytes: usage.DownloadBytes,
		UpdatedAt:     usage.UpdatedAt.UTC(),
	}

	err := r.db.WithContext(ctx).Save(&model).Error
	if err != nil {
		return fmt.Errorf("recording token usage: %w", err)
	}
	return nil
}

// GetUsage returns traffic usage for a specific token, period type and start.
func (r *TrafficRepository) GetUsage(
	ctx context.Context, tokenID string, periodType string, periodStart time.Time,
) (domain.TokenUsage, error) {
	var model tokenUsageModel
	err := r.db.WithContext(ctx).
		Where("token_id = ? AND period_type = ? AND period_start = ?", tokenID, periodType, periodStart.UTC()).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.TokenUsage{}, nil
		}
		return domain.TokenUsage{}, fmt.Errorf("getting token usage: %w", err)
	}
	return toDomainTokenUsage(model), nil
}

// ListUsageByToken returns recent usage records for a token.
func (r *TrafficRepository) ListUsageByToken(
	ctx context.Context, tokenID string, periodType string, limit int,
) ([]domain.TokenUsage, error) {
	var models []tokenUsageModel
	err := r.db.WithContext(ctx).
		Where("token_id = ? AND period_type = ?", tokenID, periodType).
		Order("period_start DESC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing token usage: %w", err)
	}

	out := make([]domain.TokenUsage, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainTokenUsage(m))
	}
	return out, nil
}

// ResetAllForPeriod deletes all usage records for a given period.
func (r *TrafficRepository) ResetAllForPeriod(ctx context.Context, periodType string, periodStart time.Time) error {
	result := r.db.WithContext(ctx).
		Where("period_type = ? AND period_start = ?", periodType, periodStart.UTC()).
		Delete(&tokenUsageModel{})
	if result.Error != nil {
		return fmt.Errorf("resetting token usage: %w", result.Error)
	}
	return nil
}

// GetAggregateForPeriod sums upload and download across all tokens for a period.
func (r *TrafficRepository) GetAggregateForPeriod(ctx context.Context, periodType string, periodStart time.Time) (int64, int64, error) {
	var result struct {
		Upload   int64
		Download int64
	}
	err := r.db.WithContext(ctx).Model(&tokenUsageModel{}).
		Select("COALESCE(SUM(upload_bytes), 0) as upload, COALESCE(SUM(download_bytes), 0) as download").
		Where("period_type = ? AND period_start = ?", periodType, periodStart.UTC()).
		Scan(&result).Error
	if err != nil {
		return 0, 0, fmt.Errorf("aggregating token usage: %w", err)
	}
	return result.Upload, result.Download, nil
}

func toDomainTokenUsage(model tokenUsageModel) domain.TokenUsage {
	return domain.TokenUsage{
		TokenID:       model.TokenID,
		PeriodType:    model.PeriodType,
		PeriodStart:   model.PeriodStart,
		UploadBytes:   model.UploadBytes,
		DownloadBytes: model.DownloadBytes,
		UpdatedAt:     model.UpdatedAt,
	}
}

// RecordNodeUsage upserts traffic usage for a node and period.
func (r *TrafficRepository) RecordNodeUsage(ctx context.Context, usage domain.NodeUsage) error {
	model := nodeUsageModel{
		NodeID:        usage.NodeID,
		PeriodType:    usage.PeriodType,
		PeriodStart:   usage.PeriodStart.UTC(),
		UploadBytes:   usage.UploadBytes,
		DownloadBytes: usage.DownloadBytes,
		UpdatedAt:     usage.UpdatedAt.UTC(),
	}
	if err := r.db.WithContext(ctx).Save(&model).Error; err != nil {
		return fmt.Errorf("recording node usage: %w", err)
	}
	return nil
}

// GetNodeUsage returns traffic usage for a specific node, period type and start.
func (r *TrafficRepository) GetNodeUsage(
	ctx context.Context, nodeID string, periodType string, periodStart time.Time,
) (domain.NodeUsage, error) {
	var model nodeUsageModel
	err := r.db.WithContext(ctx).
		Where("node_id = ? AND period_type = ? AND period_start = ?", nodeID, periodType, periodStart.UTC()).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.NodeUsage{}, nil
		}
		return domain.NodeUsage{}, fmt.Errorf("getting node usage: %w", err)
	}
	return toDomainNodeUsage(model), nil
}

// ListNodeUsage returns usage records for nodes filtered by period.
func (r *TrafficRepository) ListNodeUsage(
	ctx context.Context, periodType string, periodStart time.Time, limit int,
) ([]domain.NodeUsage, error) {
	var models []nodeUsageModel
	q := r.db.WithContext(ctx).
		Where("period_type = ?", periodType).
		Order("upload_bytes + download_bytes DESC").
		Limit(limit)
	if !periodStart.IsZero() {
		q = q.Where("period_start = ?", periodStart.UTC())
	}
	err := q.Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing node usage: %w", err)
	}
	out := make([]domain.NodeUsage, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainNodeUsage(m))
	}
	return out, nil
}

func toDomainNodeUsage(model nodeUsageModel) domain.NodeUsage {
	return domain.NodeUsage{
		NodeID:        model.NodeID,
		PeriodType:    model.PeriodType,
		PeriodStart:   model.PeriodStart,
		UploadBytes:   model.UploadBytes,
		DownloadBytes: model.DownloadBytes,
		UpdatedAt:     model.UpdatedAt,
	}
}

// RecordInboundUsage upserts traffic usage for an inbound tag and period.
func (r *TrafficRepository) RecordInboundUsage(ctx context.Context, usage domain.InboundUsage) error {
	model := inboundUsageModel{
		InboundTag:    usage.InboundTag,
		PeriodType:    usage.PeriodType,
		PeriodStart:   usage.PeriodStart.UTC(),
		UploadBytes:   usage.UploadBytes,
		DownloadBytes: usage.DownloadBytes,
		UpdatedAt:     usage.UpdatedAt.UTC(),
	}
	if err := r.db.WithContext(ctx).Save(&model).Error; err != nil {
		return fmt.Errorf("recording inbound usage: %w", err)
	}
	return nil
}

// GetInboundUsage returns traffic usage for a specific inbound tag, period type and start.
func (r *TrafficRepository) GetInboundUsage(
	ctx context.Context, inboundTag string, periodType string, periodStart time.Time,
) (domain.InboundUsage, error) {
	var model inboundUsageModel
	err := r.db.WithContext(ctx).
		Where("inbound_tag = ? AND period_type = ? AND period_start = ?", inboundTag, periodType, periodStart.UTC()).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.InboundUsage{}, nil
		}
		return domain.InboundUsage{}, fmt.Errorf("getting inbound usage: %w", err)
	}
	return toDomainInboundUsage(model), nil
}

// ListInboundUsage returns usage records for inbounds filtered by period.
func (r *TrafficRepository) ListInboundUsage(
	ctx context.Context, periodType string, periodStart time.Time, limit int,
) ([]domain.InboundUsage, error) {
	var models []inboundUsageModel
	q := r.db.WithContext(ctx).
		Where("period_type = ?", periodType).
		Order("upload_bytes + download_bytes DESC").
		Limit(limit)
	if !periodStart.IsZero() {
		q = q.Where("period_start = ?", periodStart.UTC())
	}
	err := q.Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing inbound usage: %w", err)
	}
	out := make([]domain.InboundUsage, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainInboundUsage(m))
	}
	return out, nil
}

func toDomainInboundUsage(model inboundUsageModel) domain.InboundUsage {
	return domain.InboundUsage{
		InboundTag:    model.InboundTag,
		PeriodType:    model.PeriodType,
		PeriodStart:   model.PeriodStart,
		UploadBytes:   model.UploadBytes,
		DownloadBytes: model.DownloadBytes,
		UpdatedAt:     model.UpdatedAt,
	}
}

// ListTokenUsageForPeriod returns all token usage records for a given period.
func (r *TrafficRepository) ListTokenUsageForPeriod(
	ctx context.Context, periodType string, periodStart time.Time, limit int,
) ([]domain.TokenUsage, error) {
	var models []tokenUsageModel
	q := r.db.WithContext(ctx).
		Where("period_type = ?", periodType).
		Order("upload_bytes + download_bytes DESC").
		Limit(limit)
	if !periodStart.IsZero() {
		q = q.Where("period_start = ?", periodStart.UTC())
	}
	err := q.Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("listing token usage for period: %w", err)
	}
	out := make([]domain.TokenUsage, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainTokenUsage(m))
	}
	return out, nil
}
