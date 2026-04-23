package httpadapter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"outless/internal/app/public"
	"outless/internal/domain"
)

// GroupSyncHandler streams sync progress for a group source via SSE.
type GroupSyncHandler struct {
	groupRepo     domain.GroupRepository
	publicService *public.Service
	logger        *slog.Logger
}

func NewGroupSyncHandler(groupRepo domain.GroupRepository, publicService *public.Service, logger *slog.Logger) *GroupSyncHandler {
	return &GroupSyncHandler{
		groupRepo:     groupRepo,
		publicService: publicService,
		logger:        logger,
	}
}

// HandleStream writes SSE sync events and reports whether the request was handled.
func (h *GroupSyncHandler) HandleStream(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if !strings.HasSuffix(r.URL.Path, "/sync/stream") {
		return false
	}
	if !strings.HasPrefix(r.URL.Path, "/v1/groups/") {
		return false
	}

	groupID := strings.TrimPrefix(r.URL.Path, "/v1/groups/")
	groupID = strings.TrimSuffix(groupID, "/sync/stream")
	groupID = strings.Trim(groupID, "/")
	if groupID == "" {
		http.Error(w, "group id is required", http.StatusBadRequest)
		return true
	}

	if _, err := h.groupRepo.FindByID(r.Context(), groupID); err != nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return true
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return true
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	writeEvent := func(event string, payload any) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_ = writeEvent("started", map[string]string{"group_id": groupID})
	result, err := h.publicService.SyncGroup(r.Context(), groupID, func(event public.SyncEvent) {
		if writeErr := writeEvent("node_status", event); writeErr != nil {
			h.logger.Warn("failed to write node_status event", slog.String("error", writeErr.Error()))
		}
	})
	if err != nil {
		h.logger.Error("group sync stream failed", slog.String("group_id", groupID), slog.String("error", err.Error()))
		_ = writeEvent("error", map[string]string{"error": err.Error()})
		return true
	}

	_ = writeEvent("done", map[string]any{
		"synced_at":                 result.SyncedAt.Format(time.RFC3339),
		"deleted_unavailable_count": result.DeletedUnavailableCount,
	})
	return true
}
