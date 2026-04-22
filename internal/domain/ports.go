package domain

import (
	"context"
	"iter"
	"time"
)

// NodeRepository provides persistence operations for proxy nodes.
type NodeRepository interface {
	IterateNodes(ctx context.Context) iter.Seq2[Node, error]
	ListVLESSURLs(ctx context.Context) ([]string, error)
	UpdateProbeResult(ctx context.Context, result ProbeResult) error
}

// TokenRepository provides secure operations for subscription tokens.
type TokenRepository interface {
	IssueToken(ctx context.Context, owner string, expiresAt time.Time) (string, error)
	ValidateToken(ctx context.Context, token string, at time.Time) (bool, error)
}

// ProxyEngine validates node reachability through Xray.
type ProxyEngine interface {
	ProbeNode(ctx context.Context, node Node) (ProbeResult, error)
}

// AdminRepository provides persistence operations for admin users.
type AdminRepository interface {
	FindByUsername(ctx context.Context, username string) (Admin, error)
}
