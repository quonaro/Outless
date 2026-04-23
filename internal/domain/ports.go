package domain

import (
	"context"
	"iter"
	"time"
)

// NodeRepository provides persistence operations for proxy nodes.
type NodeRepository interface {
	IterateNodes(ctx context.Context) iter.Seq2[Node, error]
	ListVLESSURLs(ctx context.Context, groupID string) ([]string, error)
	UpdateProbeResult(ctx context.Context, result ProbeResult) error
	Create(ctx context.Context, node Node) error
	CreateIfAbsent(ctx context.Context, node Node) (bool, error)
	Upsert(ctx context.Context, node Node) error
	FindByID(ctx context.Context, id string) (Node, error)
	List(ctx context.Context) ([]Node, error)
	ListPage(ctx context.Context, limit int, offset int) ([]Node, error)
	ListByGroup(ctx context.Context, groupID string) ([]Node, error)
	ListNonHealthyByGroup(ctx context.Context, groupID string) ([]Node, error)
	DeleteUnavailableByGroup(ctx context.Context, groupID string) (int64, error)
	Update(ctx context.Context, node Node) error
	Delete(ctx context.Context, id string) error
}

// TokenRepository provides secure operations for subscription tokens.
type TokenRepository interface {
	IssueToken(ctx context.Context, owner string, groupIDs []string, expiresAt time.Time) (Token, error)
	ValidateToken(ctx context.Context, token string, at time.Time) (bool, error)
	GetTokenGroupID(ctx context.Context, token string, at time.Time) (string, error)
	GetTokenByPlain(ctx context.Context, token string, at time.Time) (Token, error)
	ListActive(ctx context.Context, at time.Time) ([]Token, error)
	List(ctx context.Context) ([]Token, error)
	Deactivate(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
}

// ProxyEngine validates node reachability through Xray.
type ProxyEngine interface {
	ProbeNode(ctx context.Context, node Node) (ProbeResult, error)
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
	UpdateSyncedAt(ctx context.Context, id string, syncedAt time.Time) error
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
