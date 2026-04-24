package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type probeJobModel struct {
	ID          string     `gorm:"column:id;primaryKey"`
	BatchID     string     `gorm:"column:batch_id"`
	NodeID      string     `gorm:"column:node_id"`
	GroupID     *string    `gorm:"column:group_id"`
	RequestedBy string     `gorm:"column:requested_by"`
	Mode        string     `gorm:"column:mode"`
	ProbeURL    string     `gorm:"column:probe_url"`
	Status      string     `gorm:"column:status"`
	Attempts    int        `gorm:"column:attempts"`
	Error       string     `gorm:"column:error"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	StartedAt   *time.Time `gorm:"column:started_at"`
	FinishedAt  *time.Time `gorm:"column:finished_at"`
}

func (probeJobModel) TableName() string {
	return "probe_jobs"
}

// GormProbeJobRepository stores async probe jobs using PostgreSQL.
type GormProbeJobRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewGormProbeJobRepository constructs a job repository backed by GORM.
func NewGormProbeJobRepository(db *gorm.DB, logger *slog.Logger) *GormProbeJobRepository {
	return &GormProbeJobRepository{db: db, logger: logger}
}

func (r *GormProbeJobRepository) EnqueueNode(ctx context.Context, in domain.ProbeJobCreate) (domain.ProbeJob, error) {
	job := probeJobModel{
		ID:          newProbeJobID(),
		BatchID:     strings.TrimSpace(in.BatchID),
		NodeID:      strings.TrimSpace(in.NodeID),
		GroupID:     nullableString(strings.TrimSpace(in.GroupID)),
		RequestedBy: strings.TrimSpace(in.RequestedBy),
		Mode:        normalizeProbeMode(in.Mode),
		ProbeURL:    strings.TrimSpace(in.ProbeURL),
		Status:      string(domain.ProbeJobStatusPending),
		Attempts:    0,
		Error:       "",
		CreatedAt:   time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&job).Error; err != nil {
		return domain.ProbeJob{}, fmt.Errorf("enqueueing probe job: %w", err)
	}
	return toDomainProbeJob(job), nil
}

func (r *GormProbeJobRepository) EnqueueBatch(ctx context.Context, jobs []domain.ProbeJobCreate) ([]domain.ProbeJob, error) {
	if len(jobs) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	models := make([]probeJobModel, 0, len(jobs))
	for _, in := range jobs {
		models = append(models, probeJobModel{
			ID:          newProbeJobID(),
			BatchID:     strings.TrimSpace(in.BatchID),
			NodeID:      strings.TrimSpace(in.NodeID),
			GroupID:     nullableString(strings.TrimSpace(in.GroupID)),
			RequestedBy: strings.TrimSpace(in.RequestedBy),
			Mode:        normalizeProbeMode(in.Mode),
			ProbeURL:    strings.TrimSpace(in.ProbeURL),
			Status:      string(domain.ProbeJobStatusPending),
			Attempts:    0,
			Error:       "",
			CreatedAt:   now,
		})
	}
	if err := r.db.WithContext(ctx).Create(&models).Error; err != nil {
		return nil, fmt.Errorf("enqueueing probe jobs batch: %w", err)
	}

	out := make([]domain.ProbeJob, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainProbeJob(m))
	}
	return out, nil
}

func (r *GormProbeJobRepository) ClaimPending(ctx context.Context, limit int) ([]domain.ProbeJob, error) {
	if limit <= 0 {
		limit = 1
	}

	claimed := make([]probeJobModel, 0, limit)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := `
UPDATE probe_jobs
SET status = ?, attempts = attempts + 1, started_at = NOW(), finished_at = NULL, error = ''
WHERE id IN (
	SELECT id
	FROM probe_jobs
	WHERE status = ?
	ORDER BY created_at
	LIMIT ?
	FOR UPDATE SKIP LOCKED
)
RETURNING id, batch_id, node_id, group_id, requested_by, mode, probe_url, status, attempts, error, created_at, started_at, finished_at`
		if err := tx.Raw(query, string(domain.ProbeJobStatusRunning), string(domain.ProbeJobStatusPending), limit).Scan(&claimed).Error; err != nil {
			return fmt.Errorf("claiming pending probe jobs: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]domain.ProbeJob, 0, len(claimed))
	for _, m := range claimed {
		out = append(out, toDomainProbeJob(m))
	}
	return out, nil
}

func (r *GormProbeJobRepository) MarkSucceeded(ctx context.Context, id string) error {
	tx := r.db.WithContext(ctx).
		Model(&probeJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      string(domain.ProbeJobStatusSucceeded),
			"error":       "",
			"finished_at": time.Now().UTC(),
		})
	if tx.Error != nil {
		return fmt.Errorf("marking probe job succeeded: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("marking probe job succeeded: %w", domain.ErrProbeJobNotFound)
	}
	return nil
}

func (r *GormProbeJobRepository) MarkFailed(ctx context.Context, id string, reason string) error {
	// Retry policy:
	// - attempts < 3: put job back to pending queue
	// - attempts >= 3: mark failed terminally
	// attempts are incremented in ClaimPending.
	failedAt := time.Now().UTC()
	tx := r.db.WithContext(ctx).
		Model(&probeJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status": gorm.Expr("CASE WHEN attempts < 3 THEN ? ELSE ? END",
				string(domain.ProbeJobStatusPending),
				string(domain.ProbeJobStatusFailed),
			),
			"error":       strings.TrimSpace(reason),
			"started_at":  gorm.Expr("CASE WHEN attempts < 3 THEN NULL ELSE started_at END"),
			"finished_at": gorm.Expr("CASE WHEN attempts < 3 THEN NULL ELSE ? END", failedAt),
		})
	if tx.Error != nil {
		return fmt.Errorf("marking probe job failed: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("marking probe job failed: %w", domain.ErrProbeJobNotFound)
	}
	return nil
}

func (r *GormProbeJobRepository) GetByID(ctx context.Context, id string) (domain.ProbeJob, error) {
	var m probeJobModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ProbeJob{}, fmt.Errorf("finding probe job by id: %w", domain.ErrProbeJobNotFound)
		}
		return domain.ProbeJob{}, fmt.Errorf("finding probe job by id: %w", err)
	}
	return toDomainProbeJob(m), nil
}

func (r *GormProbeJobRepository) List(ctx context.Context, filter domain.ProbeJobListFilter) ([]domain.ProbeJob, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := r.db.WithContext(ctx).Model(&probeJobModel{}).Order("created_at DESC").Limit(limit)
	if status := strings.TrimSpace(string(filter.Status)); status != "" {
		query = query.Where("status = ?", status)
	}
	if groupID := strings.TrimSpace(filter.GroupID); groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}

	var models []probeJobModel
	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing probe jobs: %w", err)
	}

	out := make([]domain.ProbeJob, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainProbeJob(m))
	}
	return out, nil
}

func toDomainProbeJob(m probeJobModel) domain.ProbeJob {
	return domain.ProbeJob{
		ID:          m.ID,
		BatchID:     m.BatchID,
		NodeID:      m.NodeID,
		GroupID:     derefString(m.GroupID),
		RequestedBy: m.RequestedBy,
		Mode:        domain.ProbeMode(m.Mode),
		ProbeURL:    m.ProbeURL,
		Status:      domain.ProbeJobStatus(m.Status),
		Attempts:    m.Attempts,
		LastError:   m.Error,
		CreatedAt:   m.CreatedAt,
		StartedAt:   m.StartedAt,
		FinishedAt:  m.FinishedAt,
	}
}

func normalizeProbeMode(mode domain.ProbeMode) string {
	if mode == domain.ProbeModeFast {
		return string(domain.ProbeModeFast)
	}
	return string(domain.ProbeModeNormal)
}

func newProbeJobID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("probe_job_%d", time.Now().UTC().UnixNano())
	}
	return "probe_job_" + hex.EncodeToString(b[:])
}
