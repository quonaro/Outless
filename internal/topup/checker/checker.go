package checker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"outless/internal/country"
	"outless/internal/domain"
	"outless/shared/vless"
)

// Checker validates VLESS URLs through configurable stages.
type Checker struct {
	logger          *slog.Logger
	countryResolver *country.Resolver
}

// New creates a new Checker with the provided logger and an optional country resolver.
func New(logger *slog.Logger, resolver *country.Resolver) *Checker {
	return &Checker{logger: logger, countryResolver: resolver}
}

// Run validates the given URLs concurrently using the provided configuration.
func (c *Checker) Run(ctx context.Context, urls []string, cfg domain.TopUpCheckConfig) ([]Result, error) {
	cfg = withDefaults(cfg)

	results := make([]Result, len(urls))
	sem := make(chan struct{}, cfg.Workers)
	var wg sync.WaitGroup

	for i, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, url string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = c.check(ctx, url, cfg)
		}(i, u)
	}
	wg.Wait()

	return results, nil
}

func (c *Checker) check(ctx context.Context, raw string, cfg domain.TopUpCheckConfig) Result {
	parsed, err := vless.ParseURL(raw)
	if err != nil {
		return Result{URL: raw, Err: fmt.Errorf("parse: %w", err)}
	}

	r := Result{URL: raw}
	for _, stage := range cfg.Stages {
		if err := ctx.Err(); err != nil {
			r.Err = err
			return r
		}

		r.Stage = stage
		switch stage {
		case "port":
			if err := checkTCP(ctx, parsed, cfg.Timeout); err != nil {
				r.Err = fmt.Errorf("port: %w", err)
				return r
			}
		case "handshake":
			if err := checkHandshake(ctx, parsed, cfg.Timeout); err != nil {
				r.Err = fmt.Errorf("handshake: %w", err)
				return r
			}
		case "proxy":
			if err := c.checkProxyStage(ctx, &r, parsed, cfg); err != nil {
				r.Err = err
				return r
			}
		}
	}

	return r
}

func (c *Checker) checkProxyStage(ctx context.Context, r *Result, parsed vless.Parsed, cfg domain.TopUpCheckConfig) error {
	pr, err := checkProxy(ctx, parsed, cfg.Timeout)
	if err != nil {
		return fmt.Errorf("proxy: %w", err)
	}
	r.Latency = pr.Latency
	r.RealIP = pr.RealIP
	if c.countryResolver != nil && r.RealIP != "" {
		resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		info, err := c.countryResolver.Lookup(resolveCtx, r.RealIP)
		cancel()
		if err == nil {
			r.Country = info.CountryCode
		} else {
			c.logger.Warn("failed to resolve country for proxy result", slog.String("ip", r.RealIP), slog.String("error", err.Error()))
		}
	}
	if cfg.MaxLatency > 0 && r.Latency > cfg.MaxLatency {
		return fmt.Errorf("latency %s exceeds max %s", r.Latency, cfg.MaxLatency)
	}
	if isExcluded(r.Country, cfg.ExcludeCountries) {
		return fmt.Errorf("country %s excluded", r.Country)
	}
	return nil
}

func withDefaults(cfg domain.TopUpCheckConfig) domain.TopUpCheckConfig {
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if len(cfg.Stages) == 0 {
		cfg.Stages = []string{"port", "handshake"}
	}
	return cfg
}

func isExcluded(country string, excluded []string) bool {
	if country == "" {
		return false
	}
	for _, e := range excluded {
		if strings.EqualFold(country, e) {
			return true
		}
	}
	return false
}
