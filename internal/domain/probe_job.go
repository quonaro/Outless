package domain

import "time"

// ProbeJobStatus describes probe job lifecycle state.
type ProbeJobStatus string

const (
	ProbeJobStatusPending   ProbeJobStatus = "pending"
	ProbeJobStatusRunning   ProbeJobStatus = "running"
	ProbeJobStatusSucceeded ProbeJobStatus = "succeeded"
	ProbeJobStatusFailed    ProbeJobStatus = "failed"
)

// ProbeJob is an async probe task executed by checker.
type ProbeJob struct {
	ID          string
	BatchID     string
	NodeID      string
	GroupID     string
	RequestedBy string
	Mode        ProbeMode
	ProbeURL    string
	Status      ProbeJobStatus
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
}

// ProbeJobCreate holds data required to enqueue a new probe job.
type ProbeJobCreate struct {
	BatchID     string
	NodeID      string
	GroupID     string
	RequestedBy string
	Mode        ProbeMode
	ProbeURL    string
}

// ProbeJobListFilter contains list constraints for job browsing.
type ProbeJobListFilter struct {
	Status  ProbeJobStatus
	GroupID string
	Limit   int
}
