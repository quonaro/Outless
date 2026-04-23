package nodeprobe

import (
	"context"
	"time"

	"outless/internal/domain"
)

// ProbeWithEngine runs Xray-backed probe and falls back to unhealthy + preserved country when the engine fails.
func ProbeWithEngine(ctx context.Context, engine domain.ProxyEngine, node domain.Node) domain.ProbeResult {
	result, err := engine.ProbeNode(ctx, node)
	if err != nil {
		return domain.ProbeResult{
			NodeID:    node.ID,
			Status:    domain.NodeStatusUnhealthy,
			Country:   domain.NormalizeCountryCode(node.Country),
			Latency:   0,
			CheckedAt: time.Now().UTC(),
		}
	}
	if result.NodeID == "" {
		result.NodeID = node.ID
	}
	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now().UTC()
	}
	result.Country = domain.NormalizeCountryCode(result.Country)
	return result
}
