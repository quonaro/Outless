package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"outless/internal/domain"
)

// Engine probes nodes by asking Xray to route a temporary outbound request.
type Engine struct {
	client    *http.Client
	logger    *slog.Logger
	probeURL  string
	xrayAdmin string
}

// NewEngine constructs an Xray-backed proxy engine.
func NewEngine(client *http.Client, logger *slog.Logger, probeURL, xrayAdmin string) *Engine {
	return &Engine{
		client:    client,
		logger:    logger,
		probeURL:  probeURL,
		xrayAdmin: xrayAdmin,
	}
}

// ProbeNode checks node connectivity using generate_204 endpoint.
func (e *Engine) ProbeNode(ctx context.Context, node domain.Node) (domain.ProbeResult, error) {
	body, err := json.Marshal(map[string]string{
		"node_url": node.URL,
		"target":   e.probeURL,
	})
	if err != nil {
		return domain.ProbeResult{}, fmt.Errorf("encoding xray probe payload: %w", err)
	}

	endpoint := strings.TrimRight(e.xrayAdmin, "/") + "/probe"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return domain.ProbeResult{}, fmt.Errorf("creating probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return domain.ProbeResult{}, fmt.Errorf("probing node %s via xray: %w", node.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return domain.ProbeResult{}, fmt.Errorf("unexpected probe status for node %s: %d", node.ID, resp.StatusCode)
	}

	var probeResponse struct {
		LatencyMS int64  `json:"latency_ms"`
		Country   string `json:"country"`
	}

	if decodeErr := json.NewDecoder(resp.Body).Decode(&probeResponse); decodeErr != nil {
		return domain.ProbeResult{}, fmt.Errorf("decoding xray probe response for node %s: %w", node.ID, decodeErr)
	}

	latency := time.Duration(probeResponse.LatencyMS) * time.Millisecond
	if latency <= 0 {
		latency = time.Millisecond
	}
	e.logger.Debug("xray probe success", slog.String("node_id", node.ID), slog.Duration("latency", latency))

	country := node.Country
	if probeResponse.Country != "" {
		country = probeResponse.Country
	}

	return domain.ProbeResult{
		NodeID:    node.ID,
		Latency:   latency,
		Status:    domain.NodeStatusHealthy,
		Country:   country,
		CheckedAt: time.Now().UTC(),
	}, nil
}
