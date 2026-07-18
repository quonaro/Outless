package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"outless/internal/service"
)

// TopUpStreamHandler serves an SSE stream of top-up progress snapshots.
type TopUpStreamHandler struct {
	scheduler *service.TopUpScheduler
	logger    *slog.Logger
	interval  time.Duration
}

// NewTopUpStreamHandler creates a new SSE top-up progress handler.
func NewTopUpStreamHandler(scheduler *service.TopUpScheduler, logger *slog.Logger) *TopUpStreamHandler {
	return &TopUpStreamHandler{
		scheduler: scheduler,
		logger:    logger,
		interval:  25 * time.Second,
	}
}

func (h *TopUpStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		http.Error(w, "scheduler not available", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	ch := h.scheduler.SubscribeProgress()
	defer h.scheduler.UnsubscribeProgress(ch)

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case p, ok := <-ch:
			if !ok {
				return
			}
			if err := writeTopUpProgress(w, flusher, p); err != nil {
				h.logger.Debug("top-up stream write error", slog.String("error", err.Error()))
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeTopUpProgress(w http.ResponseWriter, flusher http.Flusher, p service.TopUpProgress) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
