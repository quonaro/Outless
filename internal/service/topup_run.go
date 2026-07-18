package service

import (
	"context"
	"log/slog"

	"outless/internal/domain"
	"outless/internal/topup/checker"
)

func (s *TopUpScheduler) runTopUp(ctx context.Context, topUp domain.GroupTopUp) (TopUpRunResult, error) {
	result := TopUpRunResult{TopUpID: topUp.ID, GroupID: topUp.GroupID}
	logger := s.logger.With(slog.String("top_up_id", topUp.ID), slog.String("group_id", topUp.GroupID))
	logger.Info("starting top-up run")
	logger.Debug("top-up configuration",
		slog.Bool("check_enabled", topUp.CheckEnabled),
		slog.Int("url_count", len(topUp.URLs)),
		slog.String("parser_type", topUp.ParserType),
	)

	base := TopUpProgress{TopUpID: topUp.ID, GroupID: topUp.GroupID, GroupName: s.groupName(ctx, topUp.GroupID)}
	s.broadcast(base.clone(TopUpStatusRunning, TopUpStageFetching, 0, 0, 0, 0, ""))

	rawURLs, err := s.fetchAndParseURLs(ctx, topUp)
	if err != nil {
		logger.Error("failed to fetch or parse urls", slog.String("error", err.Error()))
		s.broadcast(base.clone(TopUpStatusFailed, TopUpStageIdle, 0, 0, 0, 0, err.Error()))
		s.finishRun(ctx, topUp, false)
		result.Failed = true
		result.Error = err.Error()
		return result, err
	}

	result.Total = len(rawURLs)
	logger.Debug("urls fetched and parsed", slog.Int("total", result.Total))
	s.broadcast(base.clone(TopUpStatusRunning, TopUpStageChecking, result.Total, 0, 0, 0, ""))

	urls, err := s.runTopUpCheck(ctx, topUp, rawURLs, base, &result)
	if err != nil {
		s.finishRun(ctx, topUp, false)
		result.Failed = true
		result.Error = err.Error()
		return result, err
	}

	logger.Debug("replacing group nodes", slog.String("group_id", topUp.GroupID), slog.Int("urls", len(urls)))
	added, err := s.replaceGroupNodes(ctx, topUp.GroupID, urls)
	result.Added = added
	if err != nil {
		logger.Error("failed to replace group nodes", slog.String("error", err.Error()))
		s.broadcast(base.clone(TopUpStatusFailed, TopUpStageIdle, result.Total, result.Checked, result.Passed, 0, err.Error()))
		s.finishRun(ctx, topUp, false)
		result.Failed = true
		result.Error = err.Error()
		return result, err
	}

	logger.Info("top-up run completed",
		slog.Int("total", result.Total),
		slog.Int("passed", result.Passed),
		slog.Int("added", result.Added),
	)
	logger.Debug("top-up run finished", slog.Bool("failed", result.Failed))
	s.broadcast(base.clone(TopUpStatusCompleted, TopUpStageIdle, result.Total, result.Checked, result.Passed, result.Added, ""))
	s.finishRun(ctx, topUp, true)
	return result, nil
}

func (s *TopUpScheduler) runTopUpCheck(
	ctx context.Context,
	topUp domain.GroupTopUp,
	rawURLs []string,
	base TopUpProgress,
	result *TopUpRunResult,
) ([]string, error) {
	if !topUp.CheckEnabled {
		result.Checked = len(rawURLs)
		result.Passed = len(rawURLs)
		return rawURLs, nil
	}

	logger := s.logger.With(slog.String("top_up_id", topUp.ID))
	logger.Info("running checker", slog.Int("urls", len(rawURLs)))
	logger.Debug("checker configuration",
		slog.Int("workers", topUp.CheckConfig.Workers),
		slog.String("timeout", topUp.CheckConfig.Timeout.String()),
		slog.Any("stages", topUp.CheckConfig.Stages),
	)

	onProgress := func(p checker.Progress) {
		s.broadcast(toTopUpProgress(p, base))
	}
	results, err := s.checker.Run(ctx, rawURLs, topUp.CheckConfig, onProgress)
	if err != nil {
		s.broadcast(base.clone(TopUpStatusFailed, TopUpStageIdle, result.Total, 0, 0, 0, err.Error()))
		return nil, err
	}

	urls := make([]string, 0, len(results))
	for _, r := range results {
		if r.Passed() {
			urls = append(urls, r.URL)
		}
	}
	result.Passed = len(urls)
	result.Checked = result.Total
	s.broadcast(base.clone(TopUpStatusRunning, TopUpStageImporting, result.Total, result.Checked, result.Passed, 0, ""))
	return urls, nil
}
