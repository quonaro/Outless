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

	syncMu      sync.Mutex
	activeSyncs map[string]*syncRun // group_id -> running sync

	publicRefreshMu      sync.Mutex
	publicRefreshLastRun *time.Time
	publicRefreshNextRun *time.Time

	statePath   string
	persistMu   sync.Mutex
	lastPersist time.Time
}

type syncRun struct {
	cancel context.CancelFunc

	mu         sync.Mutex
	running    bool
	total      int
	processed  int
	addedCount int
	syncedAt   string
	error      string
	finishedAt time.Time
	nodes      map[string]syncNodeState
}

type syncNodeState struct {
	NodeID string `json:"node_id"`
	URL    string `json:"url"`
	Error  string `json:"error,omitempty"`
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
	h.syncMu.Unlock()
	if hasSync {
		syncRun.cancel()
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
			NodeID: ev.NodeID,
			URL:    ev.URL,
			Error:  ev.Error,
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
	run.addedCount = result.AddedCount
	run.finishedAt = time.Now().UTC()
	processed := run.processed
	total := run.total
	added := run.addedCount
	run.mu.Unlock()
	h.persistSnapshotMaybe(true)

	h.broadcastJSON(map[string]any{
		"type":        "sync_done",
		"group_id":    groupID,
		"synced_at":   result.SyncedAt.Format(time.RFC3339),
		"processed":   processed,
		"total":       total,
		"added_count": added,
	})
	h.NotifyInvalidate(true, true)
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
		"type":        "sync_group_state",
		"group_id":    groupID,
		"running":     run.running,
		"processed":   run.processed,
		"total":       run.total,
		"nodes":       nodes,
		"error":       run.error,
		"synced_at":   run.syncedAt,
		"added_count": run.addedCount,
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
	Version int                        `json:"version"`
	Syncs   map[string]syncRunSnapshot `json:"syncs"`
}

type syncRunSnapshot struct {
	Running    bool            `json:"running"`
	Total      int             `json:"total"`
	Processed  int             `json:"processed"`
	AddedCount int             `json:"added_count"`
	SyncedAt   string          `json:"synced_at,omitempty"`
	Error      string          `json:"error,omitempty"`
	FinishedAt time.Time       `json:"finished_at,omitempty"`
	Nodes      []syncNodeState `json:"nodes"`
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
	for groupID, s := range snap.Syncs {
		run := &syncRun{
			running:    s.Running,
			total:      s.Total,
			processed:  s.Processed,
			addedCount: s.AddedCount,
			syncedAt:   s.SyncedAt,
			error:      s.Error,
			finishedAt: s.FinishedAt,
			nodes:      make(map[string]syncNodeState, len(s.Nodes)),
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
	h.logger.Info("realtime snapshot restored",
		slog.String("path", h.statePath),
		slog.Int("sync_groups", len(snap.Syncs)),
	)
}

func (h *RealtimeHandler) persistSnapshotMaybe(force bool) {
	if h.statePath == "" {
		return
	}
	h.persistMu.Lock()
	now := time.Now()
	// Debounce: don't write more frequently than every 2 seconds unless forced
	if !force && !h.lastPersist.IsZero() && now.Sub(h.lastPersist) < 2*time.Second {
		h.persistMu.Unlock()
		return
	}
	h.lastPersist = now
	h.persistMu.Unlock()

	snap := realtimeSnapshot{
		Version: 1,
		Syncs:   make(map[string]syncRunSnapshot),
	}

	h.syncMu.Lock()
	for groupID, run := range h.activeSyncs {
		run.mu.Lock()
		nodes := make([]syncNodeState, 0, len(run.nodes))
		for _, n := range run.nodes {
			nodes = append(nodes, n)
		}
		snap.Syncs[groupID] = syncRunSnapshot{
			Running:    run.running,
			Total:      run.total,
			Processed:  run.processed,
			AddedCount: run.addedCount,
			SyncedAt:   run.syncedAt,
			Error:      run.error,
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
