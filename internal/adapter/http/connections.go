package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"outless/internal/domain"
)

// ConnectionsHandler serves active sing-box connections.
type ConnectionsHandler struct {
	runtime domain.RuntimeController
	logger  *slog.Logger
}

// NewConnectionsHandler constructs a connections handler.
func NewConnectionsHandler(runtime domain.RuntimeController, logger *slog.Logger) *ConnectionsHandler {
	return &ConnectionsHandler{runtime: runtime, logger: logger}
}

// connectionItem is a single connection for the API response.
type connectionItem struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	NodeID   string `json:"node_id"`
	Inbound  string `json:"inbound"`
	Domain   string `json:"domain"`
	SourceIP string `json:"source_ip"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// connectionsResponse holds the snapshot data.
type connectionsResponse struct {
	UploadTotal   int64            `json:"upload_total"`
	DownloadTotal int64            `json:"download_total"`
	Connections   []connectionItem `json:"connections"`
}

// Register wires the connections endpoint manually (not Huma) to support SSE.
func (h *ConnectionsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/connections", h.handleConnections)
}

func (h *ConnectionsHandler) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	snap := h.runtime.TrafficSnapshot()
	if snap == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(connectionsResponse{})
		return
	}

	items := make([]connectionItem, 0, len(snap.Connections))
	for _, c := range snap.Connections {
		items = append(items, connectionItem{
			ID:       c.ID,
			User:     c.User,
			NodeID:   c.NodeID,
			Inbound:  c.Inbound,
			Domain:   c.Domain,
			SourceIP: c.SourceIP,
			Upload:   c.Upload,
			Download: c.Download,
		})
	}

	resp := connectionsResponse{
		UploadTotal:   snap.UploadTotal,
		DownloadTotal: snap.DownloadTotal,
		Connections:   items,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode connections", slog.String("error", err.Error()))
	}
}

// StreamConnectionsHandler serves an SSE stream of connection snapshots.
type StreamConnectionsHandler struct {
	runtime  domain.RuntimeController
	interval time.Duration
	logger   *slog.Logger
}

// NewStreamConnectionsHandler creates an SSE connections handler.
func NewStreamConnectionsHandler(runtime domain.RuntimeController, logger *slog.Logger) *StreamConnectionsHandler {
	return &StreamConnectionsHandler{runtime: runtime, interval: 3 * time.Second, logger: logger}
}

func (h *StreamConnectionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		snap := h.runtime.TrafficSnapshot()
		items := make([]connectionItem, 0, len(snap.Connections))
		for _, c := range snap.Connections {
			items = append(items, connectionItem{
				ID:       c.ID,
				User:     c.User,
				NodeID:   c.NodeID,
				Inbound:  c.Inbound,
				Domain:   c.Domain,
				SourceIP: c.SourceIP,
				Upload:   c.Upload,
				Download: c.Download,
			})
		}
		resp := connectionsResponse{
			UploadTotal:   snap.UploadTotal,
			DownloadTotal: snap.DownloadTotal,
			Connections:   items,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			h.logger.Error("failed to marshal connections", slog.String("error", err.Error()))
			return
		}

		_, err = w.Write([]byte("data: "))
		if err != nil {
			return
		}
		_, err = w.Write(data)
		if err != nil {
			return
		}
		_, err = w.Write([]byte("\n\n"))
		if err != nil {
			return
		}
		flusher.Flush()

		select {
		case <-ticker.C:
		case <-r.Context().Done():
			return
		}
	}
}
