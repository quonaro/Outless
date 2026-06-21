package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"outless/internal/domain"

	"gorm.io/gorm"
)

type inboundModel struct {
	ID                 string    `gorm:"column:id;primaryKey"`
	Name               string    `gorm:"column:name"`
	Address            string    `gorm:"column:address"`
	Port               int64     `gorm:"column:port"`
	SNI                string    `gorm:"column:sni"`
	Handshake          string    `gorm:"column:handshake"`
	PublicKey          string    `gorm:"column:public_key"`
	PrivateKey         string    `gorm:"column:private_key"`
	ShortID            string    `gorm:"column:short_id"`
	Fingerprint        string    `gorm:"column:fingerprint"`
	URLHost            string    `gorm:"column:url_host"`
	NameTemplate       string    `gorm:"column:name_template"`
	EnableAutoSelfNode bool      `gorm:"column:enable_auto_self_node"`
	AutoSelfNodeName   string    `gorm:"column:auto_self_node_name"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (inboundModel) TableName() string { return "inbounds" }

// InboundRepository persists VLESS REALITY inbounds using GORM over SQLite.
type InboundRepository struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewInboundRepository constructs a GORM-backed inbound repository.
func NewInboundRepository(db *gorm.DB, logger *slog.Logger) *InboundRepository {
	return &InboundRepository{db: db, logger: logger}
}

func (r *InboundRepository) Create(ctx context.Context, inbound domain.Inbound) error {
	model := inboundModel{
		ID:                 inbound.ID,
		Name:               inbound.Name,
		Address:            inbound.Address,
		Port:               int64(inbound.Port),
		SNI:                inbound.SNI,
		Handshake:          inbound.Handshake,
		PublicKey:          inbound.PublicKey,
		PrivateKey:         inbound.PrivateKey,
		ShortID:            inbound.ShortID,
		Fingerprint:        inbound.Fingerprint,
		URLHost:            inbound.URLHost,
		NameTemplate:       inbound.NameTemplate,
		EnableAutoSelfNode: inbound.EnableAutoSelfNode,
		AutoSelfNodeName:   inbound.AutoSelfNodeName,
		CreatedAt:          inbound.CreatedAt,
		UpdatedAt:          inbound.UpdatedAt,
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now().UTC()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = model.CreatedAt
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("creating inbound: %w", err)
	}
	r.logger.Info("inbound created", slog.String("id", model.ID), slog.String("name", model.Name))
	return nil
}

func (r *InboundRepository) FindByID(ctx context.Context, id string) (domain.Inbound, error) {
	var model inboundModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Inbound{}, fmt.Errorf("inbound not found: %w", domain.ErrInboundNotFound)
		}
		return domain.Inbound{}, fmt.Errorf("finding inbound by id: %w", err)
	}
	return toDomainInbound(model), nil
}

func (r *InboundRepository) List(ctx context.Context) ([]domain.Inbound, error) {
	var models []inboundModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("listing inbounds: %w", err)
	}
	inbounds := make([]domain.Inbound, 0, len(models))
	for _, model := range models {
		inbounds = append(inbounds, toDomainInbound(model))
	}
	return inbounds, nil
}

func (r *InboundRepository) Update(ctx context.Context, inbound domain.Inbound) error {
	updates := map[string]any{
		"name":                 inbound.Name,
		"address":              inbound.Address,
		"port":                 int64(inbound.Port),
		"sni":                  inbound.SNI,
		"handshake":            inbound.Handshake,
		"public_key":           inbound.PublicKey,
		"private_key":          inbound.PrivateKey,
		"short_id":             inbound.ShortID,
		"fingerprint":          inbound.Fingerprint,
		"url_host":             inbound.URLHost,
		"name_template":        inbound.NameTemplate,
		"enable_auto_self_node": inbound.EnableAutoSelfNode,
		"auto_self_node_name":  inbound.AutoSelfNodeName,
		"updated_at":           time.Now().UTC(),
	}
	result := r.db.WithContext(ctx).Model(&inboundModel{}).Where("id = ?", inbound.ID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("updating inbound: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("inbound not found: %w", domain.ErrInboundNotFound)
	}
	r.logger.Info("inbound updated", slog.String("id", inbound.ID))
	return nil
}

func (r *InboundRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&inboundModel{})
	if result.Error != nil {
		return fmt.Errorf("deleting inbound: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("inbound not found: %w", domain.ErrInboundNotFound)
	}
	r.logger.Info("inbound deleted", slog.String("id", id))
	return nil
}

func toDomainInbound(model inboundModel) domain.Inbound {
	return domain.Inbound{
		ID:                 model.ID,
		Name:               model.Name,
		Address:            model.Address,
		Port:               int(model.Port),
		SNI:                model.SNI,
		Handshake:          model.Handshake,
		PublicKey:          model.PublicKey,
		PrivateKey:         model.PrivateKey,
		ShortID:            model.ShortID,
		Fingerprint:        model.Fingerprint,
		URLHost:            model.URLHost,
		NameTemplate:       model.NameTemplate,
		EnableAutoSelfNode: model.EnableAutoSelfNode,
		AutoSelfNodeName:   model.AutoSelfNodeName,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}
