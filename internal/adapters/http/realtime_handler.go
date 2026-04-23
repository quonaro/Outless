package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"outless/internal/app/public"
	"outless/internal/domain"

	"github.com/coder/websocket"
)

// RealtimeHandler serves a single WebSocket for admin UI: group sync progress and cache invalidation hints.
type RealtimeHandler struct {
	public *public.Service
	groups domain.GroupRepository
	logger *slog.Logger

	clientsMu sync.Mutex
	clients   []*wsClient

	syncMu       sync.Mutex
	activeSyncs  map[string]*syncRun  // group_id -> running sync
	activeProbes map[string]*probeRun // group_id -> running unavailable probe
}

type syncRun struct {
	cancel context.CancelFunc

	mu                      sync.Mutex
	running                 bool
	total                   int
	processed               int
	deletedUnavailableCount int64
	syncedAt                string
	error                   string
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

	mu        sync.Mutex
	running   bool
	total     int
	processed int
	nodes     map[string]probeNodeState
}

type probeNodeState struct {
	NodeID     string `json:"node_id"`
	URL        string `json:"url"`
	Status     string `json:"status"`
	LatencyMS  int64  `json:"latency_ms"`
	NodeStatus string `json:"node_status,omitempty"`
	Error      string `json:"error,omitempty"`
}

type wsClient struct {
	conn    *websocket.Conn
	rootCtx context.Context
	writeMu sync.Mutex
}

// NewRealtimeHandler constructs the realtime WebSocket handler.
func NewRealtimeHandler(public *public.Service, groups domain.GroupRepository, logger *slog.Logger) *RealtimeHandler {
	return &RealtimeHandler{
		public:       public,
		groups:       groups,
		logger:       logger,
		activeSyncs:  make(map[string]*syncRun),
		activeProbes: make(map[string]*probeRun),
	}
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

	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return true
		}
		var msg struct {
			Action  string `json:"action"`
			GroupID string `json:"group_id"`
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
			go h.runGroupProbeUnavailable(client, msg.GroupID)
		case "probe_unavailable_state":
			if msg.GroupID == "" {
				_ = client.writeJSON(r.Context(), map[string]string{"type": "error", "error": "group_id is required"})
				continue
			}
			h.sendProbeUnavailableState(client, msg.GroupID)
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
	if run, busy := h.activeSyncs[groupID]; busy {
		h.syncMu.Unlock()
		h.sendSyncGroupStateFromRun(client, groupID, run)
		return
	}
	run := &syncRun{
		cancel:    cancel,
		running:   true,
		total:     0,
		processed: 0,
		nodes:     make(map[string]syncNodeState),
	}
	h.activeSyncs[groupID] = run
	h.syncMu.Unlock()

	defer func() {
		h.syncMu.Lock()
		if current, ok := h.activeSyncs[groupID]; ok && current == run {
			delete(h.activeSyncs, groupID)
		}
		h.syncMu.Unlock()
	}()

	h.broadcastJSON(map[string]any{
		"type":      "sync_started",
		"group_id":  groupID,
		"processed": 0,
		"total":     0,
	})

	writeNode := func(ev public.SyncEvent) {
		run.mu.Lock()
		prev, hadPrev := run.nodes[ev.NodeID]
		state := syncNodeState{
			NodeID:    ev.NodeID,
			URL:       ev.URL,
			Status:    string(ev.Status),
			LatencyMS: ev.LatencyMS,
			Error:     ev.Error,
		}
		run.nodes[ev.NodeID] = state
		run.total = len(run.nodes)
		if isSyncTerminal(string(ev.Status)) && (!hadPrev || !isSyncTerminal(prev.Status)) {
			run.processed++
		}
		processed := run.processed
		total := run.total
		run.mu.Unlock()

		m := map[string]any{
			"type":       "sync_node_status",
			"group_id":   groupID,
			"node_id":    ev.NodeID,
			"url":        ev.URL,
			"status":     string(ev.Status),
			"latency_ms": ev.LatencyMS,
			"processed":  processed,
			"total":      total,
		}
		if ev.Error != "" {
			m["error"] = ev.Error
		}
		h.broadcastJSON(m)
	}

	result, err := h.public.SyncGroup(syncCtx, groupID, writeNode)
	if err != nil {
		run.mu.Lock()
		run.running = false
		run.error = err.Error()
		processed := run.processed
		total := run.total
		run.mu.Unlock()

		if errors.Is(err, context.Canceled) {
			h.broadcastJSON(map[string]any{
				"type":      "sync_cancelled",
				"group_id":  groupID,
				"processed": processed,
				"total":     total,
			})
		} else {
			h.broadcastJSON(map[string]any{
				"type":      "sync_error",
				"group_id":  groupID,
				"error":     err.Error(),
				"processed": processed,
				"total":     total,
			})
		}
		h.NotifyInvalidate(true, true)
		return
	}

	run.mu.Lock()
	run.running = false
	run.syncedAt = result.SyncedAt.Format(time.RFC3339)
	run.deletedUnavailableCount = result.DeletedUnavailableCount
	processed := run.processed
	total := run.total
	run.mu.Unlock()

	h.broadcastJSON(map[string]any{
		"type":                      "sync_done",
		"group_id":                  groupID,
		"synced_at":                 result.SyncedAt.Format(time.RFC3339),
		"deleted_unavailable_count": result.DeletedUnavailableCount,
		"processed":                 processed,
		"total":                     total,
	})
	h.NotifyInvalidate(true, true)
}

func (h *RealtimeHandler) runGroupProbeUnavailable(client *wsClient, groupID string) {
	ctx := context.Background()
	if _, err := h.groups.FindByID(ctx, groupID); err != nil {
		_ = client.writeJSON(client.rootCtx, map[string]any{"type": "probe_unavailable_error", "group_id": groupID, "error": "group not found"})
		return
	}

	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	h.syncMu.Lock()
	if run, busy := h.activeProbes[groupID]; busy {
		h.syncMu.Unlock()
		h.sendProbeUnavailableStateFromRun(client, groupID, run)
		return
	}
	total, countErr := h.public.CountUnavailableByGroup(probeCtx, groupID)
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
		nodes:     make(map[string]probeNodeState, total),
	}
	h.activeProbes[groupID] = run
	h.syncMu.Unlock()

	defer func() {
		h.syncMu.Lock()
		if current, ok := h.activeProbes[groupID]; ok && current == run {
			delete(h.activeProbes, groupID)
		}
		h.syncMu.Unlock()
	}()

	h.broadcastJSON(map[string]any{
		"type":      "probe_unavailable_started",
		"group_id":  groupID,
		"total":     total,
		"processed": 0,
	})

	writeProbeNode := func(ev public.ProbeUnavailableEvent) {
		run.mu.Lock()
		state := probeNodeState{
			NodeID:     ev.NodeID,
			URL:        ev.URL,
			Status:     string(ev.Status),
			LatencyMS:  ev.LatencyMS,
			NodeStatus: ev.NodeStatus,
			Error:      ev.Error,
		}
		run.nodes[ev.NodeID] = state
		if ev.Status == public.ProbeUnavailableNodeStatusReady || ev.Status == public.ProbeUnavailableNodeStatusError {
			run.processed++
		}
		processed := run.processed
		total := run.total
		run.mu.Unlock()

		m := map[string]any{
			"type":       "probe_unavailable_node_status",
			"group_id":   groupID,
			"node_id":    ev.NodeID,
			"url":        ev.URL,
			"status":     string(ev.Status),
			"latency_ms": ev.LatencyMS,
			"processed":  processed,
			"total":      total,
		}
		if ev.Error != "" {
			m["error"] = ev.Error
		}
		if ev.NodeStatus != "" {
			m["node_status"] = ev.NodeStatus
		}
		h.broadcastJSON(m)
	}

	probed, err := h.public.ProbeUnavailableGroup(probeCtx, groupID, writeProbeNode)
	if err != nil {
		run.mu.Lock()
		run.running = false
		processed := run.processed
		total := run.total
		run.mu.Unlock()

		if errors.Is(err, context.Canceled) {
			h.broadcastJSON(map[string]any{
				"type":      "probe_unavailable_cancelled",
				"group_id":  groupID,
				"processed": processed,
				"total":     total,
			})
		} else {
			h.broadcastJSON(map[string]any{
				"type":      "probe_unavailable_error",
				"group_id":  groupID,
				"error":     err.Error(),
				"processed": processed,
				"total":     total,
			})
		}
		h.NotifyInvalidate(true, true)
		return
	}

	run.mu.Lock()
	run.running = false
	processed := run.processed
	total = run.total
	run.mu.Unlock()

	h.broadcastJSON(map[string]any{
		"type":      "probe_unavailable_done",
		"group_id":  groupID,
		"probed":    probed,
		"processed": processed,
		"total":     total,
	})
	h.NotifyInvalidate(true, true)
}

func (h *RealtimeHandler) sendProbeUnavailableState(client *wsClient, groupID string) {
	h.syncMu.Lock()
	run, ok := h.activeProbes[groupID]
	h.syncMu.Unlock()
	if !ok {
		_ = client.writeJSON(client.rootCtx, map[string]any{
			"type":      "probe_unavailable_state",
			"group_id":  groupID,
			"running":   false,
			"total":     0,
			"processed": 0,
			"nodes":     []probeNodeState{},
		})
		return
	}
	h.sendProbeUnavailableStateFromRun(client, groupID, run)
}

func (h *RealtimeHandler) sendProbeUnavailableStateFromRun(client *wsClient, groupID string, run *probeRun) {
	run.mu.Lock()
	nodes := make([]probeNodeState, 0, len(run.nodes))
	for _, n := range run.nodes {
		nodes = append(nodes, n)
	}
	payload := map[string]any{
		"type":      "probe_unavailable_state",
		"group_id":  groupID,
		"running":   run.running,
		"total":     run.total,
		"processed": run.processed,
		"nodes":     nodes,
	}
	run.mu.Unlock()
	_ = client.writeJSON(client.rootCtx, payload)
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
	}
	run.mu.Unlock()
	_ = client.writeJSON(client.rootCtx, payload)
}

func isSyncTerminal(status string) bool {
	return status == "done" || status == "unavailable" || status == "error"
}
