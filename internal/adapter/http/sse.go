package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"outless/internal/domain"
	"outless/internal/service"
)

// SSEHandler serves Server-Sent Events for admin UI: group sync progress and cache invalidation hints.
type SSEHandler struct {
	public *service.PublicService
	groups domain.GroupRepository
	logger *slog.Logger

	clientsMu sync.Mutex
	clients   map[string]*sseClient
	clientSeq int

	syncMu      sync.Mutex
	activeSyncs map[string]*syncRun

	statePath   string
	persistMu   sync.Mutex
	lastPersist time.Time
}

type sseClient struct {
	ch   chan []byte
	done chan struct{}
}

// NewSSEHandler constructs the SSE handler, restoring any persisted sync snapshot.
func NewSSEHandler(
	public *service.PublicService,
	groups domain.GroupRepository,
	statePath string,
	logger *slog.Logger,
) *SSEHandler {
	h := &SSEHandler{
		public:      public,
		groups:      groups,
		logger:      logger,
		activeSyncs: make(map[string]*syncRun),
		statePath:   strings.TrimSpace(statePath),
		clients:     make(map[string]*sseClient),
	}
	h.loadSnapshot()
	return h
}

// NotifyInvalidate broadcasts a lightweight hint so clients refresh TanStack Query caches.
func (h *SSEHandler) NotifyInvalidate(nodes, groups bool) {
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
	h.broadcastRaw(payload)
}

func (h *SSEHandler) broadcastRaw(b []byte) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	for _, c := range h.clients {
		select {
		case c.ch <- b:
		default:
		}
	}
}

func (h *SSEHandler) broadcastJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.broadcastRaw(b)
}

// RegisterSSERoutes wires SSE endpoint and sync command REST endpoints.
func (h *SSEHandler) RegisterSSERoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/events", h.handleEvents)
	mux.HandleFunc("POST /v1/groups/{id}/sync", h.handleSyncGroup)
	mux.HandleFunc("POST /v1/groups/{id}/sync/cancel", h.handleCancelSync)
	mux.HandleFunc("GET /v1/groups/{id}/sync/state", h.handleSyncGroupState)
}

// handleEvents upgrades to SSE and streams events to the client.
func (h *SSEHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Warn("streaming unsupported")
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	client := &sseClient{ch: make(chan []byte, 32), done: make(chan struct{})}

	h.clientsMu.Lock()
	h.clientSeq++
	clientID := fmt.Sprintf("client-%d", h.clientSeq)
	h.clients[clientID] = client
	h.clientsMu.Unlock()

	defer func() {
		h.clientsMu.Lock()
		delete(h.clients, clientID)
		h.clientsMu.Unlock()
		close(client.done)
	}()

	h.writeEvent(w, flusher, map[string]any{"type": "welcome", "version": 1})

	for {
		select {
		case msg := <-client.ch:
			h.writeEvent(w, flusher, msg)
		case <-r.Context().Done():
			return
		case <-client.done:
			return
		}
	}
}

func (h *SSEHandler) writeEvent(w http.ResponseWriter, flusher http.Flusher, payload any) {
	var b []byte
	switch v := payload.(type) {
	case []byte:
		b = v
	default:
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			return
		}
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
	flusher.Flush()
}

func (h *SSEHandler) handleSyncGroup(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	if groupID == "" {
		http.Error(w, `{"error":"group_id is required"}`, http.StatusBadRequest)
		return
	}

	go h.runGroupSync(groupID)
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (h *SSEHandler) handleCancelSync(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	if groupID == "" {
		http.Error(w, `{"error":"group_id is required"}`, http.StatusBadRequest)
		return
	}

	h.syncMu.Lock()
	syncRun, hasSync := h.activeSyncs[groupID]
	h.syncMu.Unlock()
	if hasSync {
		syncRun.cancel()
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "canceled"})
}

func (h *SSEHandler) handleSyncGroupState(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	if groupID == "" {
		http.Error(w, `{"error":"group_id is required"}`, http.StatusBadRequest)
		return
	}

	h.syncMu.Lock()
	run, ok := h.activeSyncs[groupID]
	h.syncMu.Unlock()
	if !ok {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":      "sync_group_state",
			"group_id":  groupID,
			"running":   false,
			"processed": 0,
			"total":     0,
			"nodes":     []syncNodeState{},
			"error":     "",
			"synced_at": "",
		})
		return
	}

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
	_ = json.NewEncoder(w).Encode(payload)
}

// --- Sync logic ---

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

//nolint:funlen
func (h *SSEHandler) runGroupSync(groupID string) {
	ctx := context.Background()
	if _, err := h.groups.FindByID(ctx, groupID); err != nil {
		h.broadcastJSON(map[string]any{"type": "sync_error", "group_id": groupID, "error": "group not found"})
		return
	}

	syncCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	h.syncMu.Lock()
	if run, exists := h.activeSyncs[groupID]; exists && run.running {
		h.syncMu.Unlock()
		h.broadcastJSON(map[string]any{
			"type":      "sync_group_state",
			"group_id":  groupID,
			"running":   true,
			"processed": run.processed,
			"total":     run.total,
		})
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

	writeNode := func(ev service.SyncEvent) {
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
				"type":        "sync_canceled",
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

func isSyncTerminal(status string) bool {
	return status == "done" || status == "unavailable" || status == "error"
}

// --- Snapshot persistence ---

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

func (h *SSEHandler) loadSnapshot() {
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

func (h *SSEHandler) persistSnapshotMaybe(force bool) {
	if h.statePath == "" {
		return
	}
	h.persistMu.Lock()
	now := time.Now()
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
