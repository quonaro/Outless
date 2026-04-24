package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"outless/internal/app/public"
	"outless/internal/domain"

	"github.com/coder/websocket"
)

// RealtimeHandler serves a single WebSocket for admin UI: group sync progress and cache invalidation hints.
type RealtimeHandler struct {
	public                *public.Service
	groups                domain.GroupRepository
	logger                *slog.Logger
	publicRefreshInterval time.Duration

	clientsMu sync.Mutex
	clients   []*wsClient

	syncMu       sync.Mutex
	activeSyncs  map[string]*syncRun  // group_id -> running sync
	activeProbes map[string]*probeRun // group_id -> running unavailable probe

	publicRefreshMu      sync.Mutex
	publicRefreshLastRun *time.Time
	publicRefreshNextRun *time.Time

	statePath   string
	persistMu   sync.Mutex
	lastPersist time.Time
}

type syncRun struct {
	cancel context.CancelFunc

	mu                      sync.Mutex
	running                 bool
	total                   int
	processed               int
	addedCount              int
	deletedUnavailableCount int64
	syncedAt                string
	error                   string
	finishedAt              time.Time
	nodes                   map[string]syncNodeState
}

type syncNodeState struct {
	NodeID    string `json:"node_id"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type probeRun struct {
	cancel context.CancelFunc

	mu         sync.Mutex
	running    bool
	total      int
	processed  int
	active     int
	queued     int
	ready      int
	failed     int
	error      string
	statuses   []string
	mode       string
	probeURL   string
	startedAt  time.Time
	finishedAt time.Time
	nodes      map[string]probeNodeState
}

type probeNodeState struct {
	NodeID     string `json:"node_id"`
	URL        string `json:"url"`
	Status     string `json:"status"`
	LatencyMS  int64  `json:"latency_ms"`
	NodeStatus string `json:"node_status,omitempty"`
	Country    string `json:"country,omitempty"`
	Error      string `json:"error,omitempty"`
}

// GroupProbeUnavailableState is the payload shape for probe_unavailable_state (WS) and GET .../probe-unavailable-state (REST).
type GroupProbeUnavailableState struct {
	Running    bool             `json:"running"`
	Total      int              `json:"total"`
	Processed  int              `json:"processed"`
	Active     int              `json:"active"`
	Completed  int              `json:"completed"`
	RatePerSec float64          `json:"rate_per_sec"`
	EtaSec     int64            `json:"eta_sec"`
	Nodes      []probeNodeState `json:"nodes"`
	Error      string           `json:"error,omitempty"`
	Statuses   []string         `json:"statuses,omitempty"`
	Mode       string           `json:"mode,omitempty"`
	ProbeURL   string           `json:"probe_url,omitempty"`
}

func groupProbeStateToWSMap(groupID string, s GroupProbeUnavailableState) map[string]any {
	return map[string]any{
		"type":         "probe_unavailable_state",
		"group_id":     groupID,
		"running":      s.Running,
		"total":        s.Total,
		"processed":    s.Processed,
		"active":       s.Active,
		"completed":    s.Completed,
		"nodes":        s.Nodes,
		"error":        s.Error,
		"statuses":     s.Statuses,
		"mode":         s.Mode,
		"probe_url":    s.ProbeURL,
		"rate_per_sec": s.RatePerSec,
		"eta_sec":      s.EtaSec,
	}
}

func idleGroupProbeUnavailableState() GroupProbeUnavailableState {
	return GroupProbeUnavailableState{
		EtaSec: -1,
		Nodes:  []probeNodeState{},
	}
}

// ProbeUnavailableStateForGroup returns current bulk unavailable-probe progress for a group (for REST and WS replies).
func (h *RealtimeHandler) ProbeUnavailableStateForGroup(groupID string) GroupProbeUnavailableState {
	h.syncMu.Lock()
	run, ok := h.activeProbes[groupID]
	h.syncMu.Unlock()
	if !ok {
		return idleGroupProbeUnavailableState()
	}
	return h.groupProbeUnavailableStateFromRun(run)
}

func (h *RealtimeHandler) groupProbeUnavailableStateFromRun(run *probeRun) GroupProbeUnavailableState {
	run.mu.Lock()
	defer run.mu.Unlock()
	nodes := make([]probeNodeState, 0, len(run.nodes))
	for _, n := range run.nodes {
		nodes = append(nodes, n)
	}
	rate, eta := probeRateAndETA(run.startedAt, run.total, run.ready+run.failed)
	return GroupProbeUnavailableState{
		Running:    run.running,
		Total:      run.total,
		Processed:  run.processed,
		Active:     run.active,
		Completed:  run.ready + run.failed,
		RatePerSec: rate,
		EtaSec:     eta,
		Nodes:      nodes,
		Error:      run.error,
		Statuses:   append([]string(nil), run.statuses...),
		Mode:       run.mode,
		ProbeURL:   run.probeURL,
	}
}

type wsClient struct {
	conn    *websocket.Conn
	rootCtx context.Context
	writeMu sync.Mutex
}

// NewRealtimeHandler constructs the realtime WebSocket handler.
func NewRealtimeHandler(
	public *public.Service,
	groups domain.GroupRepository,
	publicRefreshInterval time.Duration,
	statePath string,
	logger *slog.Logger,
) *RealtimeHandler {
	h := &RealtimeHandler{
		public:                public,
		groups:                groups,
		logger:                logger,
		publicRefreshInterval: publicRefreshInterval,
		activeSyncs:           make(map[string]*syncRun),
		activeProbes:          make(map[string]*probeRun),
		statePath:             strings.TrimSpace(statePath),
	}
	h.loadSnapshot()
	return h
}

// NotifyInvalidate broadcasts a lightweight hint so clients refresh TanStack Query caches.
func (h *RealtimeHandler) NotifyInvalidate(nodes, groups bool) {
	if h == nil {
		return
	}
	keys := make([]string, 0, 2)
	if nodes {
		keys = append(keys, "nodes")
	}
	if groups {
		keys = append(keys, "groups")
	}
	if len(keys) == 0 {
		return
	}
	payload, err := json.Marshal(map[string]any{"type": "invalidate", "keys": keys})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.clientsMu.Lock()
	clients := append([]*wsClient(nil), h.clients...)
	h.clientsMu.Unlock()

	for _, c := range clients {
		c.writeMu.Lock()
		_ = c.conn.Write(ctx, websocket.MessageText, payload)
		c.writeMu.Unlock()
	}
}

func (c *wsClient) writeJSON(ctx context.Context, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	wctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.conn.Write(wctx, websocket.MessageText, b)
}

// HandleWebSocket upgrades to WebSocket and handles client messages. Returns true if handled.
func (h *RealtimeHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/v1/ws" {
		return false
	}
	if r.Method != http.MethodGet {
		return false
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.logger.Warn("websocket accept failed", slog.String("error", err.Error()))
		return true
	}

	client := &wsClient{conn: conn, rootCtx: r.Context()}

	h.clientsMu.Lock()
	h.clients = append(h.clients, client)
	h.clientsMu.Unlock()

	defer func() {
		h.clientsMu.Lock()
		out := h.clients[:0]
		for _, c := range h.clients {
			if c != client {
				out = append(out, c)
			}
		}
		h.clients = out
		h.clientsMu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "bye")
	}()

	_ = client.writeJSON(r.Context(), map[string]any{"type": "welcome", "version": 1})
	h.sendPublicRefreshState(client)

	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return true
		}
		var msg struct {
			Action   string   `json:"action"`
			GroupID  string   `json:"group_id"`
			Statuses []string `json:"statuses"`
			Mode     string   `json:"mode"`
			ProbeURL string   `json:"probe_url"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = client.writeJSON(r.Context(), map[string]string{"type": "error", "error": "invalid json"})
			continue
		}
		switch msg.Action {
		case "ping":
			_ = client.writeJSON(r.Context(), map[string]string{"type": "pong"})
		case "sync_group":
			if msg.GroupID == "" {
				_ = client.writeJSON(r.Context(), map[string]string{"type": "error", "error": "group_id is required"})
				continue
			}
			go h.runGroupSync(client, msg.GroupID)
		case "sync_group_state":
			if msg.GroupID == "" {
				_ = client.writeJSON(r.Context(), map[string]string{"type": "error", "error": "group_id is required"})
				continue
			}
			h.sendSyncGroupState(client, msg.GroupID)
		case "cancel_sync":
			if msg.GroupID == "" {
				continue
			}
			h.cancelGroupJobs(msg.GroupID)
		case "probe_unavailable":
			if msg.GroupID == "" {
				_ = client.writeJSON(r.Context(), map[string]string{"type": "error", "error": "group_id is required"})
				continue
			}
			statuses := normalizeProbeStatuses(msg.Statuses)
			mode := normalizeProbeMode(msg.Mode)
			go h.runGroupProbeUnavailable(client, msg.GroupID, statuses, mode, msg.ProbeURL)
		case "probe_unavailable_state":
			if msg.GroupID == "" {
				_ = client.writeJSON(r.Context(), map[string]string{"type": "error", "error": "group_id is required"})
				continue
			}
			h.sendProbeUnavailableState(client, msg.GroupID)
		case "public_refresh_state":
			h.sendPublicRefreshState(client)
		default:
			_ = client.writeJSON(r.Context(), map[string]string{"type": "error", "error": "unknown action"})
		}
	}
}

func (h *RealtimeHandler) cancelGroupJobs(groupID string) {
	h.syncMu.Lock()
	syncRun, hasSync := h.activeSyncs[groupID]
	probeRun, hasProbe := h.activeProbes[groupID]
	h.syncMu.Unlock()
	if hasSync {
		syncRun.cancel()
	}
	if hasProbe {
		probeRun.cancel()
	}
}

func (h *RealtimeHandler) runGroupSync(client *wsClient, groupID string) {
	ctx := context.Background()
	if _, err := h.groups.FindByID(ctx, groupID); err != nil {
		_ = client.writeJSON(client.rootCtx, map[string]any{"type": "sync_error", "group_id": groupID, "error": "group not found"})
		return
	}

	syncCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	h.syncMu.Lock()
	if run, exists := h.activeSyncs[groupID]; exists && run.running {
		h.syncMu.Unlock()
		h.sendSyncGroupStateFromRun(client, groupID, run)
		return
	}
	run := &syncRun{
		cancel:     cancel,
		running:    true,
		total:      0,
		processed:  0,
		addedCount: 0,
		nodes:      make(map[string]syncNodeState),
	}
	h.activeSyncs[groupID] = run
	h.syncMu.Unlock()
	h.persistSnapshotMaybe(true)

	h.broadcastJSON(map[string]any{
		"type":      "sync_started",
		"group_id":  groupID,
		"processed": 0,
		"total":     0,
	})

	writeNode := func(ev public.SyncEvent) {
		run.mu.Lock()
		state := syncNodeState{
			NodeID:    ev.NodeID,
			URL:       ev.URL,
			Status:    string(ev.Status),
			LatencyMS: ev.LatencyMS,
			Error:     ev.Error,
		}
		run.nodes[ev.NodeID] = state
		if isSyncTerminal(string(ev.Status)) {
			run.processed++
		}
		if ev.AddedTotal > run.addedCount {
			run.addedCount = ev.AddedTotal
		}
		processed := run.processed
		total := run.total
		added := run.addedCount
		run.mu.Unlock()
		h.persistSnapshotMaybe(false)

		m := map[string]any{
			"type":        "sync_node_status",
			"group_id":    groupID,
			"node_id":     ev.NodeID,
			"url":         ev.URL,
			"status":      string(ev.Status),
			"latency_ms":  ev.LatencyMS,
			"processed":   processed,
			"total":       total,
			"added_total": added,
		}
		if ev.Error != "" {
			m["error"] = ev.Error
		}
		h.broadcastJSON(m)
	}

	setTotal := func(total int) {
		run.mu.Lock()
		run.total = total
		processed := run.processed
		run.mu.Unlock()
		h.persistSnapshotMaybe(false)
		h.broadcastJSON(map[string]any{
			"type":      "sync_started",
			"group_id":  groupID,
			"processed": processed,
			"total":     total,
		})
	}

	result, err := h.public.SyncGroup(syncCtx, groupID, setTotal, writeNode)
	if err != nil {
		run.mu.Lock()
		run.running = false
		run.error = err.Error()
		run.finishedAt = time.Now().UTC()
		processed := run.processed
		total := run.total
		added := run.addedCount
		run.mu.Unlock()
		h.persistSnapshotMaybe(true)

		if errors.Is(err, context.Canceled) {
			h.broadcastJSON(map[string]any{
				"type":        "sync_cancelled",
				"group_id":    groupID,
				"processed":   processed,
				"total":       total,
				"added_count": added,
			})
		} else {
			h.broadcastJSON(map[string]any{
				"type":        "sync_error",
				"group_id":    groupID,
				"error":       err.Error(),
				"processed":   processed,
				"total":       total,
				"added_count": added,
			})
		}
		h.NotifyInvalidate(true, true)
		return
	}

	run.mu.Lock()
	run.running = false
	run.syncedAt = result.SyncedAt.Format(time.RFC3339)
	run.deletedUnavailableCount = result.DeletedUnavailableCount
	run.addedCount = result.AddedCount
	run.finishedAt = time.Now().UTC()
	processed := run.processed
	total := run.total
	added := run.addedCount
	run.mu.Unlock()
	h.persistSnapshotMaybe(true)

	h.broadcastJSON(map[string]any{
		"type":                      "sync_done",
		"group_id":                  groupID,
		"synced_at":                 result.SyncedAt.Format(time.RFC3339),
		"deleted_unavailable_count": result.DeletedUnavailableCount,
		"processed":                 processed,
		"total":                     total,
		"added_count":               added,
	})
	h.NotifyInvalidate(true, true)
}

func (h *RealtimeHandler) runGroupProbeUnavailable(client *wsClient, groupID string, statuses []domain.NodeStatus, mode domain.ProbeMode, probeURL string) {
	ctx := context.Background()
	if _, err := h.groups.FindByID(ctx, groupID); err != nil {
		_ = client.writeJSON(client.rootCtx, map[string]any{"type": "probe_unavailable_error", "group_id": groupID, "error": "group not found"})
		return
	}

	probeCtx, cancel := context.WithCancel(ctx)
	probeCtx = domain.WithProbeMode(probeCtx, mode)
	probeCtx = domain.WithProbeURL(probeCtx, probeURL)
	defer cancel()

	h.syncMu.Lock()
	if run, exists := h.activeProbes[groupID]; exists && run.running {
		h.syncMu.Unlock()
		h.sendProbeUnavailableStateFromRun(client, groupID, run)
		return
	}
	total, countErr := h.public.CountUnavailableByGroup(probeCtx, groupID, statuses)
	if countErr != nil {
		h.syncMu.Unlock()
		_ = client.writeJSON(client.rootCtx, map[string]any{"type": "probe_unavailable_error", "group_id": groupID, "error": countErr.Error()})
		return
	}
	run := &probeRun{
		cancel:    cancel,
		running:   true,
		total:     total,
		processed: 0,
		active:    0,
		queued:    0,
		ready:     0,
		failed:    0,
		statuses:  probeStatusesToStrings(statuses),
		mode:      string(mode),
		probeURL:  strings.TrimSpace(probeURL),
		startedAt: time.Now().UTC(),
		nodes:     make(map[string]probeNodeState, total),
	}
	h.activeProbes[groupID] = run
	h.syncMu.Unlock()
	h.persistSnapshotMaybe(true)

	h.broadcastJSON(map[string]any{
		"type":         "probe_unavailable_started",
		"group_id":     groupID,
		"total":        total,
		"processed":    0,
		"active":       0,
		"completed":    0,
		"rate_per_sec": 0.0,
		"eta_sec":      int64(-1),
		"statuses":     run.statuses,
		"mode":         run.mode,
		"probe_url":    run.probeURL,
	})

	writeProbeNode := func(ev public.ProbeUnavailableEvent) {
		run.mu.Lock()
		prev, hadPrev := run.nodes[ev.NodeID]
		state := probeNodeState{
			NodeID:     ev.NodeID,
			URL:        ev.URL,
			Status:     string(ev.Status),
			LatencyMS:  ev.LatencyMS,
			NodeStatus: ev.NodeStatus,
			Country:    ev.Country,
			Error:      ev.Error,
		}
		run.nodes[ev.NodeID] = state
		adjustProbeNodeCounters(run, hadPrev, prev.Status, state.Status)
		if ev.Status == public.ProbeUnavailableNodeStatusReady || ev.Status == public.ProbeUnavailableNodeStatusError {
			run.processed++
		}
		processed := run.processed
		total := run.total
		active := run.active
		completed := run.ready + run.failed
		ratePerSec, etaSec := probeRateAndETA(run.startedAt, total, completed)
		run.mu.Unlock()
		h.persistSnapshotMaybe(false)

		m := map[string]any{
			"type":         "probe_unavailable_node_status",
			"group_id":     groupID,
			"node_id":      ev.NodeID,
			"url":          ev.URL,
			"status":       string(ev.Status),
			"latency_ms":   ev.LatencyMS,
			"processed":    processed,
			"total":        total,
			"active":       active,
			"completed":    completed,
			"rate_per_sec": ratePerSec,
			"eta_sec":      etaSec,
		}
		if ev.Error != "" {
			m["error"] = ev.Error
		}
		if ev.NodeStatus != "" {
			m["node_status"] = ev.NodeStatus
		}
		if ev.Country != "" {
			m["country"] = ev.Country
		}
		h.broadcastJSON(m)
	}

	probed, err := h.public.ProbeUnavailableGroup(probeCtx, groupID, statuses, writeProbeNode)
	if err != nil {
		run.mu.Lock()
		run.running = false
		run.error = err.Error()
		run.finishedAt = time.Now().UTC()
		processed := run.processed
		total := run.total
		active := run.active
		completed := run.ready + run.failed
		ratePerSec, etaSec := probeRateAndETA(run.startedAt, total, completed)
		run.mu.Unlock()
		h.persistSnapshotMaybe(true)

		if errors.Is(err, context.Canceled) {
			h.broadcastJSON(map[string]any{
				"type":         "probe_unavailable_cancelled",
				"group_id":     groupID,
				"processed":    processed,
				"total":        total,
				"active":       active,
				"completed":    completed,
				"rate_per_sec": ratePerSec,
				"eta_sec":      etaSec,
			})
		} else {
			h.broadcastJSON(map[string]any{
				"type":         "probe_unavailable_error",
				"group_id":     groupID,
				"error":        err.Error(),
				"processed":    processed,
				"total":        total,
				"active":       active,
				"completed":    completed,
				"rate_per_sec": ratePerSec,
				"eta_sec":      etaSec,
			})
		}
		h.NotifyInvalidate(true, true)
		return
	}

	run.mu.Lock()
	run.running = false
	run.finishedAt = time.Now().UTC()
	processed := run.processed
	total = run.total
	active := run.active
	completed := run.ready + run.failed
	ratePerSec, etaSec := probeRateAndETA(run.startedAt, total, completed)
	run.mu.Unlock()
	h.persistSnapshotMaybe(true)

	h.broadcastJSON(map[string]any{
		"type":         "probe_unavailable_done",
		"group_id":     groupID,
		"probed":       probed,
		"processed":    processed,
		"total":        total,
		"active":       active,
		"completed":    completed,
		"rate_per_sec": ratePerSec,
		"eta_sec":      etaSec,
	})
	h.NotifyInvalidate(true, true)
}

func (h *RealtimeHandler) sendProbeUnavailableState(client *wsClient, groupID string) {
	st := h.ProbeUnavailableStateForGroup(groupID)
	_ = client.writeJSON(client.rootCtx, groupProbeStateToWSMap(groupID, st))
}

func (h *RealtimeHandler) sendProbeUnavailableStateFromRun(client *wsClient, groupID string, run *probeRun) {
	st := h.groupProbeUnavailableStateFromRun(run)
	_ = client.writeJSON(client.rootCtx, groupProbeStateToWSMap(groupID, st))
}

func (h *RealtimeHandler) broadcastJSON(v any) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.clientsMu.Lock()
	clients := append([]*wsClient(nil), h.clients...)
	h.clientsMu.Unlock()
	for _, c := range clients {
		if err := c.writeJSON(ctx, v); err != nil {
			h.logger.Debug("broadcast failed", slog.String("error", err.Error()))
		}
	}
}

func (h *RealtimeHandler) sendSyncGroupState(client *wsClient, groupID string) {
	h.syncMu.Lock()
	run, ok := h.activeSyncs[groupID]
	h.syncMu.Unlock()
	if !ok {
		_ = client.writeJSON(client.rootCtx, map[string]any{
			"type":                      "sync_group_state",
			"group_id":                  groupID,
			"running":                   false,
			"processed":                 0,
			"total":                     0,
			"nodes":                     []syncNodeState{},
			"error":                     "",
			"synced_at":                 "",
			"deleted_unavailable_count": 0,
		})
		return
	}
	h.sendSyncGroupStateFromRun(client, groupID, run)
}

func (h *RealtimeHandler) sendSyncGroupStateFromRun(client *wsClient, groupID string, run *syncRun) {
	run.mu.Lock()
	nodes := make([]syncNodeState, 0, len(run.nodes))
	for _, n := range run.nodes {
		nodes = append(nodes, n)
	}
	payload := map[string]any{
		"type":                      "sync_group_state",
		"group_id":                  groupID,
		"running":                   run.running,
		"processed":                 run.processed,
		"total":                     run.total,
		"nodes":                     nodes,
		"error":                     run.error,
		"synced_at":                 run.syncedAt,
		"deleted_unavailable_count": run.deletedUnavailableCount,
		"added_count":               run.addedCount,
	}
	run.mu.Unlock()
	_ = client.writeJSON(client.rootCtx, payload)
}

func isSyncTerminal(status string) bool {
	return status == "done" || status == "unavailable" || status == "error"
}

// UpdatePublicRefreshSchedule stores and broadcasts next public source refresh metadata.
func (h *RealtimeHandler) UpdatePublicRefreshSchedule(lastRunAt, nextRunAt *time.Time) {
	h.publicRefreshMu.Lock()
	if h.publicRefreshInterval <= 0 {
		h.publicRefreshLastRun = nil
		h.publicRefreshNextRun = nil
	} else {
		if lastRunAt != nil {
			h.publicRefreshLastRun = cloneTimePtr(lastRunAt)
		}
		if nextRunAt != nil {
			h.publicRefreshNextRun = cloneTimePtr(nextRunAt)
		}
	}
	h.publicRefreshMu.Unlock()
	h.broadcastPublicRefreshState()
}

func (h *RealtimeHandler) broadcastPublicRefreshState() {
	payload := h.publicRefreshPayload()
	h.broadcastJSON(payload)
}

func (h *RealtimeHandler) sendPublicRefreshState(client *wsClient) {
	_ = client.writeJSON(client.rootCtx, h.publicRefreshPayload())
}

func (h *RealtimeHandler) publicRefreshPayload() map[string]any {
	h.publicRefreshMu.Lock()
	lastRunAt := cloneTimePtr(h.publicRefreshLastRun)
	nextRunAt := cloneTimePtr(h.publicRefreshNextRun)
	h.publicRefreshMu.Unlock()

	payload := map[string]any{
		"type":                "public_refresh_state",
		"enabled":             h.publicRefreshInterval > 0,
		"interval_ms":         h.publicRefreshInterval.Milliseconds(),
		"server_time":         time.Now().UTC().Format(time.RFC3339),
		"last_refresh_at":     "",
		"next_refresh_at":     "",
		"next_refresh_in_ms":  int64(-1),
		"last_refresh_age_ms": int64(-1),
	}
	if lastRunAt != nil {
		payload["last_refresh_at"] = lastRunAt.UTC().Format(time.RFC3339)
		payload["last_refresh_age_ms"] = time.Since(lastRunAt.UTC()).Milliseconds()
	}
	if nextRunAt != nil {
		payload["next_refresh_at"] = nextRunAt.UTC().Format(time.RFC3339)
		payload["next_refresh_in_ms"] = time.Until(nextRunAt.UTC()).Milliseconds()
	}
	return payload
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

type realtimeSnapshot struct {
	Version int                         `json:"version"`
	Syncs   map[string]syncRunSnapshot  `json:"syncs"`
	Probes  map[string]probeRunSnapshot `json:"probes"`
}

type syncRunSnapshot struct {
	Running                 bool            `json:"running"`
	Total                   int             `json:"total"`
	Processed               int             `json:"processed"`
	AddedCount              int             `json:"added_count"`
	DeletedUnavailableCount int64           `json:"deleted_unavailable_count"`
	SyncedAt                string          `json:"synced_at,omitempty"`
	Error                   string          `json:"error,omitempty"`
	FinishedAt              time.Time       `json:"finished_at,omitempty"`
	Nodes                   []syncNodeState `json:"nodes"`
}

type probeRunSnapshot struct {
	Running    bool             `json:"running"`
	Total      int              `json:"total"`
	Processed  int              `json:"processed"`
	Error      string           `json:"error,omitempty"`
	Statuses   []string         `json:"statuses,omitempty"`
	Mode       string           `json:"mode,omitempty"`
	ProbeURL   string           `json:"probe_url,omitempty"`
	FinishedAt time.Time        `json:"finished_at,omitempty"`
	Nodes      []probeNodeState `json:"nodes"`
}

func (h *RealtimeHandler) loadSnapshot() {
	if h.statePath == "" {
		return
	}
	data, err := os.ReadFile(h.statePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			h.logger.Warn("realtime snapshot read failed", slog.String("path", h.statePath), slog.String("error", err.Error()))
		}
		return
	}
	var snap realtimeSnapshot
	if err = json.Unmarshal(data, &snap); err != nil {
		h.logger.Warn("realtime snapshot parse failed", slog.String("path", h.statePath), slog.String("error", err.Error()))
		return
	}

	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	if h.activeSyncs == nil {
		h.activeSyncs = make(map[string]*syncRun)
	}
	if h.activeProbes == nil {
		h.activeProbes = make(map[string]*probeRun)
	}
	for groupID, s := range snap.Syncs {
		run := &syncRun{
			running:                 s.Running,
			total:                   s.Total,
			processed:               s.Processed,
			addedCount:              s.AddedCount,
			deletedUnavailableCount: s.DeletedUnavailableCount,
			syncedAt:                s.SyncedAt,
			error:                   s.Error,
			finishedAt:              s.FinishedAt,
			nodes:                   make(map[string]syncNodeState, len(s.Nodes)),
		}
		if run.running {
			run.running = false
			if strings.TrimSpace(run.error) == "" {
				run.error = "interrupted by server restart"
			}
		}
		for _, n := range s.Nodes {
			run.nodes[n.NodeID] = n
		}
		h.activeSyncs[groupID] = run
	}
	for groupID, p := range snap.Probes {
		run := &probeRun{
			running:    p.Running,
			total:      p.Total,
			processed:  p.Processed,
			active:     0,
			queued:     0,
			ready:      0,
			failed:     0,
			error:      p.Error,
			statuses:   append([]string(nil), p.Statuses...),
			mode:       p.Mode,
			probeURL:   p.ProbeURL,
			startedAt:  time.Now().UTC(),
			finishedAt: p.FinishedAt,
			nodes:      make(map[string]probeNodeState, len(p.Nodes)),
		}
		if run.running {
			run.running = false
			if strings.TrimSpace(run.error) == "" {
				run.error = "interrupted by server restart"
			}
		}
		for _, n := range p.Nodes {
			run.nodes[n.NodeID] = n
			adjustProbeNodeCounters(run, false, "", n.Status)
		}
		h.activeProbes[groupID] = run
	}
	h.logger.Info("realtime snapshot restored",
		slog.String("path", h.statePath),
		slog.Int("sync_groups", len(snap.Syncs)),
		slog.Int("probe_groups", len(snap.Probes)),
	)
}

func (h *RealtimeHandler) persistSnapshotMaybe(force bool) {
	if h.statePath == "" {
		return
	}
	h.persistMu.Lock()
	now := time.Now()
	if !force && !h.lastPersist.IsZero() && now.Sub(h.lastPersist) < time.Second {
		h.persistMu.Unlock()
		return
	}
	h.lastPersist = now
	h.persistMu.Unlock()

	snap := realtimeSnapshot{
		Version: 1,
		Syncs:   make(map[string]syncRunSnapshot),
		Probes:  make(map[string]probeRunSnapshot),
	}

	h.syncMu.Lock()
	for groupID, run := range h.activeSyncs {
		run.mu.Lock()
		nodes := make([]syncNodeState, 0, len(run.nodes))
		for _, n := range run.nodes {
			nodes = append(nodes, n)
		}
		snap.Syncs[groupID] = syncRunSnapshot{
			Running:                 run.running,
			Total:                   run.total,
			Processed:               run.processed,
			AddedCount:              run.addedCount,
			DeletedUnavailableCount: run.deletedUnavailableCount,
			SyncedAt:                run.syncedAt,
			Error:                   run.error,
			FinishedAt:              run.finishedAt,
			Nodes:                   nodes,
		}
		run.mu.Unlock()
	}
	for groupID, run := range h.activeProbes {
		run.mu.Lock()
		nodes := make([]probeNodeState, 0, len(run.nodes))
		for _, n := range run.nodes {
			nodes = append(nodes, n)
		}
		snap.Probes[groupID] = probeRunSnapshot{
			Running:    run.running,
			Total:      run.total,
			Processed:  run.processed,
			Error:      run.error,
			Statuses:   append([]string(nil), run.statuses...),
			Mode:       run.mode,
			ProbeURL:   run.probeURL,
			FinishedAt: run.finishedAt,
			Nodes:      nodes,
		}
		run.mu.Unlock()
	}
	h.syncMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(h.statePath), 0o755); err != nil {
		h.logger.Warn("realtime snapshot mkdir failed", slog.String("path", h.statePath), slog.String("error", err.Error()))
		return
	}
	data, err := json.Marshal(snap)
	if err != nil {
		h.logger.Warn("realtime snapshot marshal failed", slog.String("error", err.Error()))
		return
	}
	tmp := h.statePath + ".tmp"
	if err = os.WriteFile(tmp, data, 0o600); err != nil {
		h.logger.Warn("realtime snapshot write failed", slog.String("path", h.statePath), slog.String("error", err.Error()))
		return
	}
	if err = os.Rename(tmp, h.statePath); err != nil {
		_ = os.Remove(tmp)
		h.logger.Warn("realtime snapshot rename failed", slog.String("path", h.statePath), slog.String("error", err.Error()))
	}
}

func normalizeProbeStatuses(raw []string) []domain.NodeStatus {
	if len(raw) == 0 {
		return []domain.NodeStatus{
			domain.NodeStatusUnknown,
			domain.NodeStatusUnhealthy,
			domain.NodeStatusHealthy,
		}
	}
	out := make([]domain.NodeStatus, 0, len(raw))
	seen := make(map[domain.NodeStatus]struct{}, len(raw))
	for _, item := range raw {
		switch domain.NodeStatus(strings.ToLower(strings.TrimSpace(item))) {
		case domain.NodeStatusUnknown, domain.NodeStatusUnhealthy, domain.NodeStatusHealthy:
			st := domain.NodeStatus(strings.ToLower(strings.TrimSpace(item)))
			if _, ok := seen[st]; ok {
				continue
			}
			seen[st] = struct{}{}
			out = append(out, st)
		}
	}
	if len(out) == 0 {
		return []domain.NodeStatus{
			domain.NodeStatusUnknown,
			domain.NodeStatusUnhealthy,
			domain.NodeStatusHealthy,
		}
	}
	return out
}

func probeStatusesToStrings(statuses []domain.NodeStatus) []string {
	out := make([]string, 0, len(statuses))
	for _, st := range statuses {
		out = append(out, string(st))
	}
	return out
}

func normalizeProbeMode(raw string) domain.ProbeMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(domain.ProbeModeFast):
		return domain.ProbeModeFast
	default:
		return domain.ProbeModeNormal
	}
}

func adjustProbeNodeCounters(run *probeRun, hadPrev bool, prevStatus string, nextStatus string) {
	if hadPrev && prevStatus == nextStatus {
		return
	}
	if hadPrev {
		decrementProbeStatusCounter(run, prevStatus)
	}
	incrementProbeStatusCounter(run, nextStatus)
}

func incrementProbeStatusCounter(run *probeRun, status string) {
	switch status {
	case string(public.ProbeUnavailableNodeStatusQueued):
		run.queued++
	case string(public.ProbeUnavailableNodeStatusProbing):
		run.active++
	case string(public.ProbeUnavailableNodeStatusReady):
		run.ready++
	case string(public.ProbeUnavailableNodeStatusError):
		run.failed++
	}
}

func decrementProbeStatusCounter(run *probeRun, status string) {
	switch status {
	case string(public.ProbeUnavailableNodeStatusQueued):
		if run.queued > 0 {
			run.queued--
		}
	case string(public.ProbeUnavailableNodeStatusProbing):
		if run.active > 0 {
			run.active--
		}
	case string(public.ProbeUnavailableNodeStatusReady):
		if run.ready > 0 {
			run.ready--
		}
	case string(public.ProbeUnavailableNodeStatusError):
		if run.failed > 0 {
			run.failed--
		}
	}
}

func probeRateAndETA(startedAt time.Time, total int, completed int) (float64, int64) {
	if startedAt.IsZero() || completed <= 0 {
		return 0, -1
	}
	elapsed := time.Since(startedAt).Seconds()
	if elapsed <= 0 {
		return 0, -1
	}
	rate := float64(completed) / elapsed
	if rate <= 0 {
		return 0, -1
	}
	remaining := total - completed
	if remaining <= 0 {
		return rate, 0
	}
	eta := int64(float64(remaining) / rate)
	if eta < 0 {
		eta = 0
	}
	return rate, eta
}
