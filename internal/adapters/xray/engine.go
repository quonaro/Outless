package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"outless/internal/domain"
)

// Engine probes nodes by asking Xray to route a temporary outbound request.
type Engine struct {
	client         *http.Client
	logger         *slog.Logger
	probeURL       string
	adminEndpoints []string
}

// NewEngine constructs an Xray-backed proxy engine.
func NewEngine(client *http.Client, logger *slog.Logger, probeURL, xrayAdmin string) *Engine {
	return &Engine{
		client:         client,
		logger:         logger,
		probeURL:       probeURL,
		adminEndpoints: buildAdminEndpoints(xrayAdmin),
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

	var (
		resp      *http.Response
		lastError error
	)
	for idx, adminEndpoint := range e.adminEndpoints {
		endpoint := strings.TrimRight(adminEndpoint, "/") + "/probe"
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
		if reqErr != nil {
			return domain.ProbeResult{}, fmt.Errorf("creating probe request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = e.client.Do(req)
		if err == nil {
			break
		}
		lastError = err
		if idx < len(e.adminEndpoints)-1 {
			e.logger.Debug("xray admin endpoint unavailable, trying fallback",
				slog.String("failed_endpoint", adminEndpoint),
				slog.String("fallback_endpoint", e.adminEndpoints[idx+1]),
				slog.String("error", err.Error()),
			)
			continue
		}
		e.logger.Warn("xray probe transport failed", slog.String("node_id", node.ID), slog.String("error", err.Error()))
		return domain.ProbeResult{}, fmt.Errorf("probing node %s via xray: %w", node.ID, err)
	}
	if resp == nil {
		return domain.ProbeResult{}, fmt.Errorf("probing node %s via xray: %w", node.ID, lastError)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		e.logger.Warn("xray probe rejected",
			slog.String("node_id", node.ID),
			slog.Int("http_status", resp.StatusCode),
			slog.String("body_snippet", strings.TrimSpace(string(snippet))),
		)
		return domain.ProbeResult{}, fmt.Errorf("unexpected probe status for node %s: %d", node.ID, resp.StatusCode)
	}

	var probeResponse struct {
		LatencyMS int64  `json:"latency_ms"`
		Country   string `json:"country"`
	}

	if decodeErr := json.NewDecoder(resp.Body).Decode(&probeResponse); decodeErr != nil {
		e.logger.Warn("xray probe response decode failed", slog.String("node_id", node.ID), slog.String("error", decodeErr.Error()))
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
	country = domain.NormalizeCountryCode(country)

	return domain.ProbeResult{
		NodeID:    node.ID,
		Latency:   latency,
		Status:    domain.NodeStatusHealthy,
		Country:   country,
		CheckedAt: time.Now().UTC(),
	}, nil
}

func buildAdminEndpoints(primary string) []string {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		return []string{"http://localhost:10085"}
	}

	endpoints := []string{primary}
	parsed, err := url.Parse(primary)
	if err != nil {
		return endpoints
	}
	if !strings.EqualFold(parsed.Hostname(), "xray") {
		return endpoints
	}

	port := parsed.Port()
	if port == "" {
		port = "10085"
	}
	fallbackURL := *parsed
	fallbackURL.Host = net.JoinHostPort("localhost", port)

	fallback := fallbackURL.String()
	if fallback != primary {
		endpoints = append(endpoints, fallback)
	}
	return endpoints
}
