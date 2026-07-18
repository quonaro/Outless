package http

import (
	"fmt"
	"time"

	"outless/internal/domain"
)

// TopUpInput carries the top-up configuration embedded in group create/update requests.
type TopUpInput struct {
	URLs         []string        `json:"urls"`
	ParserType   string          `json:"parser_type"`
	ParserParams map[string]any  `json:"parser_params,omitempty"`
	CheckEnabled bool            `json:"check_enabled"`
	CheckConfig  TopUpCheckInput `json:"check_config,omitempty"`
	ScheduleType string          `json:"schedule_type"`
	ScheduleExpr string          `json:"schedule_expr"`
	Enabled      bool            `json:"enabled"`
	NextRunAt    *string         `json:"next_run_at,omitempty"`
}

// TopUpCheckInput mirrors TopUpCheckConfig with string durations for JSON.
type TopUpCheckInput struct {
	Workers          int      `json:"workers"`
	Timeout          string   `json:"timeout"`
	ExcludeCountries []string `json:"exclude_countries"`
	MaxLatency       string   `json:"max_latency"`
	Stages           []string `json:"stages"`
}

// TopUpCheckOutput mirrors TopUpCheckConfig with string durations for JSON.
type TopUpCheckOutput struct {
	Workers          int      `json:"workers"`
	Timeout          string   `json:"timeout"`
	ExcludeCountries []string `json:"exclude_countries"`
	MaxLatency       string   `json:"max_latency"`
	Stages           []string `json:"stages"`
}

// TopUpOutput is the API representation of a GroupTopUp record.
type TopUpOutput struct {
	ID           string           `json:"id"`
	GroupID      string           `json:"group_id"`
	URLs         []string         `json:"urls"`
	ParserType   string           `json:"parser_type"`
	ParserParams map[string]any   `json:"parser_params"`
	CheckEnabled bool             `json:"check_enabled"`
	CheckConfig  TopUpCheckOutput `json:"check_config"`
	ScheduleType string           `json:"schedule_type"`
	ScheduleExpr string           `json:"schedule_expr"`
	NextRunAt    time.Time        `json:"next_run_at"`
	LastRunAt    *time.Time       `json:"last_run_at,omitempty"`
	Enabled      bool             `json:"enabled"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// ListTopUpsOutput returns all top-up records.
type ListTopUpsOutput struct {
	Body []TopUpOutput `json:"top_ups"`
}

// GetTopUpInput identifies a top-up by ID.
type GetTopUpInput struct {
	ID string `path:"id" required:"true"`
}

// GetTopUpOutput returns a single top-up record.
type GetTopUpOutput struct {
	Body TopUpOutput `json:"top_up"`
}

// RunTopUpInput triggers a top-up run manually.
type RunTopUpInput struct {
	ID string `path:"id" required:"true"`
}

// TopUpRunResultOutput reports the result of running a single top-up.
type TopUpRunResultOutput struct {
	TopUpID   string `json:"top_up_id"`
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Status    string `json:"status"`
	Total     int    `json:"total"`
	Passed    int    `json:"passed"`
	Added     int    `json:"added"`
	Error     string `json:"error,omitempty"`
}

// RunAllTopUpsInput triggers runs for all enabled top-ups.
type RunAllTopUpsInput struct{}

// RunAllTopUpsOutput returns the status of each top-up run.
type RunAllTopUpsOutput struct {
	Body []TopUpRunResultOutput `json:"results"`
}

// DeleteTopUpInput identifies a top-up to delete.
type DeleteTopUpInput struct {
	ID string `path:"id" required:"true"`
}

func toTopUpOutput(t domain.GroupTopUp) TopUpOutput {
	return TopUpOutput{
		ID:           t.ID,
		GroupID:      t.GroupID,
		URLs:         t.URLs,
		ParserType:   t.ParserType,
		ParserParams: t.ParserParams,
		CheckEnabled: t.CheckEnabled,
		CheckConfig:  toTopUpCheckOutput(t.CheckConfig),
		ScheduleType: t.ScheduleType,
		ScheduleExpr: t.ScheduleExpr,
		NextRunAt:    t.NextRunAt,
		LastRunAt:    t.LastRunAt,
		Enabled:      t.Enabled,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}

func toTopUpCheckOutput(cfg domain.TopUpCheckConfig) TopUpCheckOutput {
	return TopUpCheckOutput{
		Workers:          cfg.Workers,
		Timeout:          cfg.Timeout.String(),
		ExcludeCountries: cfg.ExcludeCountries,
		MaxLatency:       cfg.MaxLatency.String(),
		Stages:           cfg.Stages,
	}
}

func toTopUpCheckConfig(input TopUpCheckInput) (domain.TopUpCheckConfig, error) {
	cfg := domain.TopUpCheckConfig{
		Workers:          input.Workers,
		ExcludeCountries: input.ExcludeCountries,
		Stages:           input.Stages,
	}
	if input.Timeout != "" {
		d, err := time.ParseDuration(input.Timeout)
		if err != nil {
			return cfg, fmt.Errorf("parsing timeout: %w", err)
		}
		cfg.Timeout = d
	}
	if input.MaxLatency != "" {
		d, err := time.ParseDuration(input.MaxLatency)
		if err != nil {
			return cfg, fmt.Errorf("parsing max_latency: %w", err)
		}
		cfg.MaxLatency = d
	}
	return cfg, nil
}

func fromTopUpCheckConfig(cfg domain.TopUpCheckConfig) TopUpCheckInput {
	return TopUpCheckInput{
		Workers:          cfg.Workers,
		Timeout:          cfg.Timeout.String(),
		ExcludeCountries: cfg.ExcludeCountries,
		MaxLatency:       cfg.MaxLatency.String(),
		Stages:           cfg.Stages,
	}
}

func parseTopUpInput(input TopUpInput, groupID string) (domain.GroupTopUp, error) {
	id, err := domain.GenerateGroupTopUpID()
	if err != nil {
		return domain.GroupTopUp{}, err
	}

	cfg, err := toTopUpCheckConfig(input.CheckConfig)
	if err != nil {
		return domain.GroupTopUp{}, err
	}

	now := time.Now().UTC()
	nextRun := now
	if input.NextRunAt != nil {
		t, err := time.Parse(time.RFC3339, *input.NextRunAt)
		if err != nil {
			return domain.GroupTopUp{}, fmt.Errorf("parsing next_run_at: %w", err)
		}
		nextRun = t.UTC()
	} else if input.ScheduleType == "interval" && input.ScheduleExpr != "" {
		d, err := time.ParseDuration(input.ScheduleExpr)
		if err != nil {
			return domain.GroupTopUp{}, fmt.Errorf("parsing schedule expression: %w", err)
		}
		nextRun = now.Add(d)
	} else if input.ScheduleType == "fixed" && input.ScheduleExpr != "" {
		t, err := time.Parse(time.RFC3339, input.ScheduleExpr)
		if err != nil {
			return domain.GroupTopUp{}, fmt.Errorf("parsing fixed schedule: %w", err)
		}
		nextRun = t.UTC()
	}

	return domain.GroupTopUp{
		ID:           id,
		GroupID:      groupID,
		URLs:         input.URLs,
		ParserType:   input.ParserType,
		ParserParams: input.ParserParams,
		CheckEnabled: input.CheckEnabled,
		CheckConfig:  cfg,
		ScheduleType: input.ScheduleType,
		ScheduleExpr: input.ScheduleExpr,
		NextRunAt:    nextRun,
		Enabled:      input.Enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func mergeTopUpInput(existing domain.GroupTopUp, input TopUpInput) (domain.GroupTopUp, error) {
	existing.URLs = input.URLs
	existing.ParserType = input.ParserType
	existing.ParserParams = input.ParserParams
	existing.CheckEnabled = input.CheckEnabled
	cfg, err := toTopUpCheckConfig(input.CheckConfig)
	if err != nil {
		return existing, err
	}
	existing.CheckConfig = cfg
	existing.ScheduleType = input.ScheduleType
	existing.ScheduleExpr = input.ScheduleExpr
	existing.Enabled = input.Enabled

	if input.NextRunAt != nil {
		t, err := time.Parse(time.RFC3339, *input.NextRunAt)
		if err != nil {
			return existing, fmt.Errorf("parsing next_run_at: %w", err)
		}
		existing.NextRunAt = t.UTC()
	}

	return existing, nil
}

func topUpPtr(t domain.GroupTopUp) *TopUpOutput {
	out := toTopUpOutput(t)
	return &out
}
