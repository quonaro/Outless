package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"outless/internal/domain"
	"outless/internal/topup/parser"
	"outless/internal/utils/ssrf"
)

func (s *TopUpScheduler) fetchAndParseURLs(ctx context.Context, topUp domain.GroupTopUp) ([]string, error) {
	client := s.httpClient(topUp)
	var allURLs []string

	s.logger.Debug("fetching top-up source URLs",
		slog.Int("count", len(topUp.URLs)),
		slog.String("parser", topUp.ParserType),
	)

	timeout := topUp.CheckConfig.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	for _, u := range topUp.URLs {
		s.logger.Info("fetching top-up source URL", slog.String("url", u))
		s.logger.Debug("fetching top-up source URL request", slog.String("url", u))

		urlCtx, cancel := context.WithTimeout(ctx, timeout)
		content, err := s.fetchURL(urlCtx, client, u)
		cancel()
		if err != nil {
			s.logger.Warn("failed to fetch top-up url",
				slog.String("url", u),
				slog.String("error", err.Error()),
			)
			continue
		}

		urls, err := parser.Parse(ctx, content, topUp.ParserType, topUp.ParserParams)
		if err != nil {
			s.logger.Warn("failed to parse top-up content",
				slog.String("url", u),
				slog.String("parser", topUp.ParserType),
				slog.String("error", err.Error()),
			)
			continue
		}
		s.logger.Info("parsed top-up source URL", slog.String("url", u), slog.Int("nodes", len(urls)))
		s.logger.Debug("appending parsed top-up URLs", slog.String("url", u), slog.Int("nodes", len(urls)))
		allURLs = append(allURLs, urls...)
	}

	s.logger.Info("top-up source URLs fetched", slog.Int("sources", len(topUp.URLs)), slog.Int("total_urls", len(allURLs)))
	s.logger.Debug("top-up source URLs fetch complete", slog.Int("total_urls", len(allURLs)))
	return allURLs, nil
}

func (s *TopUpScheduler) httpClient(topUp domain.GroupTopUp) *http.Client {
	timeout := topUp.CheckConfig.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: ssrf.CheckRedirect,
	}
}

func (s *TopUpScheduler) fetchURL(ctx context.Context, client *http.Client, u string) (string, error) {
	if err := ssrf.ValidateURLWithContext(ctx, u); err != nil {
		return "", fmt.Errorf("validating URL: %w", err)
	}
	s.logger.Debug("creating HTTP request", slog.String("url", u))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	s.logger.Debug("top-up URL response", slog.String("url", u), slog.Int("status", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", err
	}
	s.logger.Debug("top-up URL body read", slog.String("url", u), slog.Int("body_len", len(body)))
	return string(body), nil
}

func (s *TopUpScheduler) replaceGroupNodes(ctx context.Context, groupID string, urls []string) (int, error) {
	s.logger.Info("replacing group nodes", slog.String("group_id", groupID), slog.Int("urls", len(urls)))

	nodes := make([]domain.Node, 0, len(urls))
	for _, u := range urls {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		nodes = append(nodes, domain.Node{
			ID:       generateTopUpNodeID(u, groupID),
			URL:      u,
			GroupIDs: []string{groupID},
		})
	}

	added, err := s.nodeRepo.ReplaceByGroupID(ctx, groupID, nodes)
	if err != nil {
		s.logger.Error("failed to replace group nodes", slog.String("group_id", groupID), slog.String("error", err.Error()))
		return 0, err
	}

	s.logger.Info("group nodes replaced", slog.String("group_id", groupID), slog.Int("added", added))
	s.logger.Debug("replace group nodes complete", slog.String("group_id", groupID), slog.Int("added", added))
	return added, nil
}

func (s *TopUpScheduler) finishRun(ctx context.Context, topUp domain.GroupTopUp, ok bool) {
	s.logger.Debug("finishing top-up run", slog.String("id", topUp.ID), slog.Bool("ok", ok))
	now := time.Now().UTC()
	topUp.LastRunAt = &now

	next, keepEnabled, err := computeNextRun(topUp, now)
	if err != nil {
		s.logger.Error("failed to compute next run", slog.String("id", topUp.ID), slog.String("error", err.Error()))
		return
	}
	topUp.NextRunAt = next
	topUp.Enabled = keepEnabled
	s.logger.Debug("computed next run", slog.String("id", topUp.ID), slog.Time("next_run_at", next), slog.Bool("enabled", keepEnabled))

	if err := s.topUpRepo.Update(ctx, topUp); err != nil {
		s.logger.Error("failed to update top-up after run", slog.String("id", topUp.ID), slog.String("error", err.Error()))
		return
	}
	s.logger.Debug("top-up record updated", slog.String("id", topUp.ID))

	if ok {
		s.logger.Info("top-up finished", slog.String("id", topUp.ID), slog.Time("next_run_at", next))
	}
}

func computeNextRun(topUp domain.GroupTopUp, base time.Time) (time.Time, bool, error) {
	if topUp.ScheduleType == "fixed" {
		return time.Time{}, false, nil
	}

	d, err := time.ParseDuration(topUp.ScheduleExpr)
	if err != nil {
		return time.Time{}, topUp.Enabled, fmt.Errorf("parsing schedule expression %q: %w", topUp.ScheduleExpr, err)
	}

	return base.Add(d), topUp.Enabled, nil
}

func generateTopUpNodeID(raw, groupID string) string {
	hash := sha256.Sum256([]byte(raw + "|" + groupID))
	return "node_" + hex.EncodeToString(hash[:8])
}
