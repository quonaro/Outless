package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

// LogBroadcaster distributes log lines to SSE subscribers.
type LogBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan string]struct{}
}

// NewLogBroadcaster creates a new broadcaster.
func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		subscribers: make(map[chan string]struct{}),
	}
}

// Subscribe returns a channel that receives log lines.
func (b *LogBroadcaster) Subscribe() chan string {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan string, 100)
	b.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a channel from the broadcaster.
func (b *LogBroadcaster) Unsubscribe(ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, ch)
	close(ch)
}

// Broadcast sends a log line to all subscribers.
func (b *LogBroadcaster) Broadcast(line string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- line:
		default:
			// channel full, drop message
		}
	}
}

// BroadcastHandler is a slog.Handler wrapper that also sends formatted lines to a broadcaster.
type BroadcastHandler struct {
	wrap   slog.Handler
	output *LogBroadcaster
	attrs  []slog.Attr
	groups []string
}

// NewBroadcastHandler wraps an existing slog.Handler.
func NewBroadcastHandler(h slog.Handler, b *LogBroadcaster) *BroadcastHandler {
	return &BroadcastHandler{wrap: h, output: b}
}

func (h *BroadcastHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.wrap.Enabled(ctx, level)
}

func (h *BroadcastHandler) Handle(ctx context.Context, r slog.Record) error {
	line := fmt.Sprintf("[%s] %s", r.Level.String(), r.Message)
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		return true
	})
	for _, a := range h.attrs {
		attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
	}
	if len(attrs) > 0 {
		line += " | " + strings.Join(attrs, " | ")
	}
	line = strings.ReplaceAll(line, "\n", " ")
	h.output.Broadcast(line)
	return h.wrap.Handle(ctx, r)
}

func (h *BroadcastHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BroadcastHandler{
		wrap:   h.wrap.WithAttrs(attrs),
		output: h.output,
		attrs:  append(h.attrs, attrs...),
		groups: h.groups,
	}
}

func (h *BroadcastHandler) WithGroup(name string) slog.Handler {
	return &BroadcastHandler{
		wrap:   h.wrap.WithGroup(name),
		output: h.output,
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
}

// LogStreamHandler serves SSE log stream.
type LogStreamHandler struct {
	broadcaster *LogBroadcaster
}

// NewLogStreamHandler creates a new SSE log handler.
func NewLogStreamHandler(b *LogBroadcaster) *LogStreamHandler {
	return &LogStreamHandler{broadcaster: b}
}

// ServeHTTP implements http.Handler for SSE log streaming.
func (h *LogStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(ch)

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
