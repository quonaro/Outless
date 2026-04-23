package domain

import (
	"context"
	"strings"
)

// ProbeMode controls optional probe behavior.
type ProbeMode string

const (
	ProbeModeNormal ProbeMode = "normal"
	ProbeModeFast   ProbeMode = "fast"
)

type probeModeContextKey struct{}
type probeURLContextKey struct{}

// WithProbeMode stores desired probe mode in context.
func WithProbeMode(ctx context.Context, mode ProbeMode) context.Context {
	if mode != ProbeModeFast {
		mode = ProbeModeNormal
	}
	return context.WithValue(ctx, probeModeContextKey{}, mode)
}

// ProbeModeFromContext returns probe mode from context, defaulting to normal.
func ProbeModeFromContext(ctx context.Context) ProbeMode {
	if ctx == nil {
		return ProbeModeNormal
	}
	if v, ok := ctx.Value(probeModeContextKey{}).(ProbeMode); ok && v == ProbeModeFast {
		return ProbeModeFast
	}
	return ProbeModeNormal
}

// IsFastProbe reports whether fast mode is requested via context.
func IsFastProbe(ctx context.Context) bool {
	return ProbeModeFromContext(ctx) == ProbeModeFast
}

// WithProbeURL stores one-off probe URL override in context.
func WithProbeURL(ctx context.Context, probeURL string) context.Context {
	probeURL = strings.TrimSpace(probeURL)
	if probeURL == "" {
		return ctx
	}
	return context.WithValue(ctx, probeURLContextKey{}, probeURL)
}

// ProbeURLFromContext returns one-off probe URL override from context.
func ProbeURLFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(probeURLContextKey{}).(string)
	return strings.TrimSpace(v)
}
