package domain

import "time"

// NodeStatus describes the latest health state of a proxy node.
type NodeStatus string

const (
	// NodeStatusUnknown means the node has not been checked yet.
	NodeStatusUnknown NodeStatus = "unknown"
	// NodeStatusHealthy means the node passed health checks.
	NodeStatusHealthy NodeStatus = "healthy"
	// NodeStatusUnhealthy means the node failed health checks.
	NodeStatusUnhealthy NodeStatus = "unhealthy"
)

// Node represents a proxy endpoint managed by Outless.
type Node struct {
	ID      string
	URL     string
	Latency time.Duration
	Status  NodeStatus
	Country string
}

// ProbeResult stores a single health-check result for a node.
type ProbeResult struct {
	NodeID   string
	Latency  time.Duration
	Status   NodeStatus
	Country  string
	CheckedAt time.Time
}
