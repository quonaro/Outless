package domain

import (
	"context"
	"iter"
	"time"
)

// NodeRepository provides persistence operations for proxy nodes.
type NodeRepository interface {
	IterateNodes(ctx context.Context) iter.Seq2[Node, error]
	ListVLESSURLs(ctx context.Context, groupID string, randomEnabled bool, randomLimit *int) ([]string, error)
	Create(ctx context.Context, node Node) error
	CreateIfAbsent(ctx context.Context, node Node) (bool, error)
	BulkCreateIfAbsent(ctx context.Context, nodes []Node) ([]string, error)
	Upsert(ctx context.Context, node Node) error
	FindByID(ctx context.Context, id string) (Node, error)
	List(ctx context.Context) ([]Node, error)
	ListPage(ctx context.Context, limit int, offset int) ([]Node, error)
	ListPageByGroup(ctx context.Context, groupID string, limit int, offset int) ([]Node, error)
	ListByGroup(ctx context.Context, groupID string) ([]Node, error)
	Update(ctx context.Context, node Node) error
	Delete(ctx context.Context, id string) error
	HasSelfNode(ctx context.Context) (bool, error)
}

// TokenRepository provides secure operations for subscription tokens.
type TokenRepository interface {
	IssueToken(
		ctx context.Context, owner string, groupIDs []string,
		inboundIDs []string, expiresAt time.Time,
		quotaBytes *int64, quotaPeriod string,
	) (Token, string, error)
	ValidateToken(ctx context.Context, token string, at time.Time) (bool, error)
	GetTokenGroupID(ctx context.Context, token string, at time.Time) (string, error)
	GetTokenByPlain(ctx context.Context, token string, at time.Time) (Token, error)
	FindByID(ctx context.Context, id string) (Token, error)
	ListActive(ctx context.Context, at time.Time) ([]Token, error)
	List(ctx context.Context) ([]Token, error)
	Deactivate(ctx context.Context, id string) error
	Activate(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	Update(
		ctx context.Context, id string, owner string,
		groupIDs []string, inboundIDs []string,
		expiresAt time.Time, quotaBytes *int64, quotaPeriod string,
	) error
	SetQuota(ctx context.Context, id string, quotaBytes *int64, quotaPeriod string) error
	CleanupExpired(ctx context.Context, cutoff time.Time) (int64, error)
}

// AdminRepository provides persistence operations for admin users.
type AdminRepository interface {
	FindByUsername(ctx context.Context, username string) (Admin, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, admin Admin) error
	List(ctx context.Context) ([]Admin, error)
	Update(ctx context.Context, admin Admin) error
	Delete(ctx context.Context, id string) error
}

// GroupRepository provides persistence operations for groups.
type GroupRepository interface {
	Create(ctx context.Context, group Group) error
	FindByID(ctx context.Context, id string) (Group, error)
	List(ctx context.Context) ([]Group, error)
	Update(ctx context.Context, group Group) error
	Delete(ctx context.Context, id string) error
}

// PublicSourceRepository provides persistence operations for public VLESS sources.
type PublicSourceRepository interface {
	Create(ctx context.Context, source PublicSource) error
	FindByID(ctx context.Context, id string) (PublicSource, error)
	List(ctx context.Context) ([]PublicSource, error)
	Update(ctx context.Context, source PublicSource) error
	Delete(ctx context.Context, id string) error
}

// InboundRepository provides persistence operations for VLESS REALITY inbounds.
type InboundRepository interface {
	Create(ctx context.Context, inbound Inbound) error
	FindByID(ctx context.Context, id string) (Inbound, error)
	List(ctx context.Context) ([]Inbound, error)
	Update(ctx context.Context, inbound Inbound) error
	Delete(ctx context.Context, id string) error
}

// TrafficConnection holds per-connection counters extracted from the runtime.
type TrafficConnection struct {
	ID       string
	User     string
	NodeID   string
	Inbound  string
	Domain   string
	Upload   int64
	Download int64
}

// TrafficSnapshot holds a point-in-time view of runtime traffic counters.
type TrafficSnapshot struct {
	UploadTotal   int64
	DownloadTotal int64
	Connections   []TrafficConnection
}

// RuntimeController abstracts how the embedded sing-box runtime is started,
// reloaded and stopped. Reload semantics are debounced close+recreate because
// sing-box has no in-place graceful reload.
type RuntimeController interface {
	Start(ctx context.Context) error
	Reload() error
	Stop()
	Description() string
	RemoveUser(email string) error
	RemoveRulesForUser(email string) error
	ForceSync() error
	TrafficSnapshot() *TrafficSnapshot
}

// NodeUsage aggregates per-node traffic for a specific period.
type NodeUsage struct {
	NodeID        string
	PeriodStart   time.Time
	PeriodType    string
	UploadBytes   int64
	DownloadBytes int64
	UpdatedAt     time.Time
}

// InboundUsage aggregates per-inbound traffic for a specific period.
type InboundUsage struct {
	InboundTag    string
	PeriodStart   time.Time
	PeriodType    string
	UploadBytes   int64
	DownloadBytes int64
	UpdatedAt     time.Time
}

// DomainUsage aggregates per-domain traffic for a specific period.
type DomainUsage struct {
	TokenID       string
	Domain        string
	PeriodStart   time.Time
	PeriodType    string
	UploadBytes   int64
	DownloadBytes int64
	UpdatedAt     time.Time
}

// TrafficRepository persists per-token traffic usage aggregates.
type TrafficRepository interface {
	RecordUsage(ctx context.Context, usage TokenUsage) error
	GetUsage(ctx context.Context, tokenID string, periodType string, periodStart time.Time) (TokenUsage, error)
	ListUsageByToken(ctx context.Context, tokenID string, periodType string, limit int) ([]TokenUsage, error)
	ResetAllForPeriod(ctx context.Context, periodType string, periodStart time.Time) error
	GetAggregateForPeriod(ctx context.Context, periodType string, periodStart time.Time) (upload int64, download int64, err error)

	RecordNodeUsage(ctx context.Context, usage NodeUsage) error
	GetNodeUsage(ctx context.Context, nodeID string, periodType string, periodStart time.Time) (NodeUsage, error)
	ListNodeUsage(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]NodeUsage, error)

	RecordInboundUsage(ctx context.Context, usage InboundUsage) error
	GetInboundUsage(ctx context.Context, inboundTag string, periodType string, periodStart time.Time) (InboundUsage, error)
	ListInboundUsage(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]InboundUsage, error)

	RecordDomainUsage(ctx context.Context, usage DomainUsage) error
	GetDomainUsage(ctx context.Context, tokenID string, domain string, periodType string, periodStart time.Time) (DomainUsage, error)
	ListDomainUsage(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]DomainUsage, error)

	ListTokenUsageForPeriod(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]TokenUsage, error)
}
