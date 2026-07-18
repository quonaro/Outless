package service

import (
	"context"
	"sync"

	"outless/internal/topup/checker"
)

// TopUpStage describes the current stage of a top-up run.
type TopUpStage string

const (
	TopUpStageFetching  TopUpStage = "fetching"
	TopUpStageChecking  TopUpStage = "checking"
	TopUpStageImporting TopUpStage = "importing"
	TopUpStageIdle      TopUpStage = "idle"
)

// TopUpStatus describes the overall status of a top-up run.
type TopUpStatus string

const (
	TopUpStatusRunning   TopUpStatus = "running"
	TopUpStatusCompleted TopUpStatus = "completed"
	TopUpStatusFailed    TopUpStatus = "failed"
	TopUpStatusIdle      TopUpStatus = "idle"
)

// TopUpProgress is a snapshot of a running or completed top-up run.
type TopUpProgress struct {
	TopUpID    string      `json:"top_up_id"`
	GroupID    string      `json:"group_id"`
	GroupName  string      `json:"group_name,omitempty"`
	Status     TopUpStatus `json:"status"`
	Stage      TopUpStage  `json:"stage"`
	Total      int         `json:"total"`
	Checked    int         `json:"checked"`
	Passed     int         `json:"passed"`
	Added      int         `json:"added"`
	Failed     int         `json:"failed"`
	CurrentURL string      `json:"current_url,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// progressBroadcaster distributes top-up progress snapshots to SSE subscribers.
type progressBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan TopUpProgress]struct{}
	last        map[string]TopUpProgress
}

func newProgressBroadcaster() *progressBroadcaster {
	return &progressBroadcaster{
		subscribers: make(map[chan TopUpProgress]struct{}),
		last:        make(map[string]TopUpProgress),
	}
}

func (pb *progressBroadcaster) Subscribe() chan TopUpProgress {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	ch := make(chan TopUpProgress, 10)
	pb.subscribers[ch] = struct{}{}

	// Replay the last known state so new subscribers see current progress immediately.
	for _, p := range pb.last {
		select {
		case ch <- p:
		default:
		}
	}
	return ch
}

func (pb *progressBroadcaster) Unsubscribe(ch chan TopUpProgress) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	delete(pb.subscribers, ch)
	close(ch)
}

func (pb *progressBroadcaster) Broadcast(p TopUpProgress) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.last[p.TopUpID] = p
	for ch := range pb.subscribers {
		select {
		case ch <- p:
		default:
		}
	}
}

func (pb *progressBroadcaster) Snapshot() map[string]TopUpProgress {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	snap := make(map[string]TopUpProgress, len(pb.last))
	for k, v := range pb.last {
		snap[k] = v
	}
	return snap
}

// SubscribeProgress returns a channel that receives progress updates for all top-up runs.
func (s *TopUpScheduler) SubscribeProgress() chan TopUpProgress {
	return s.progress.Subscribe()
}

// UnsubscribeProgress removes a progress subscriber and closes its channel.
func (s *TopUpScheduler) UnsubscribeProgress(ch chan TopUpProgress) {
	s.progress.Unsubscribe(ch)
}

// ProgressSnapshot returns the last known progress for every top-up run.
func (s *TopUpScheduler) ProgressSnapshot() map[string]TopUpProgress {
	return s.progress.Snapshot()
}

func (s *TopUpScheduler) broadcast(p TopUpProgress) {
	s.progress.Broadcast(p)
}

func (s *TopUpScheduler) groupName(ctx context.Context, groupID string) string {
	if s.groupRepo == nil || groupID == "" {
		return ""
	}
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return ""
	}
	return group.Name
}

func (p TopUpProgress) clone(status TopUpStatus, stage TopUpStage, total, checked, passed, added int, err string) TopUpProgress {
	cp := p
	cp.Status = status
	cp.Stage = stage
	cp.Total = total
	cp.Checked = checked
	cp.Passed = passed
	cp.Added = added
	cp.Error = err

	failed := checked - passed
	if failed < 0 {
		failed = 0
	}
	if checked == 0 && total > passed {
		failed = total - passed
	}
	cp.Failed = failed
	return cp
}

func toTopUpProgress(cp checker.Progress, base TopUpProgress) TopUpProgress {
	p := base.clone(TopUpStatusRunning, TopUpStageChecking, cp.Total, cp.Checked, cp.Passed, 0, "")
	p.CurrentURL = cp.Current
	return p
}
