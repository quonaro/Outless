package main

import (
	"io"
	"log/slog"
	"testing"

	"outless/pkg/config"
)

func TestBuildRuntimeController_ExternalMode(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctrl, err := buildRuntimeController(HubConfig{
		HubConfig: config.HubConfig{
			PublicKey:  "public",
			PrivateKey: "private",
			ConfigPath: "/tmp/xray-hub.json",
		},
		RuntimeMode: config.XrayRuntimeExternal,
	}, logger)
	if err != nil {
		t.Fatalf("build runtime controller: %v", err)
	}
	if got := ctrl.Description(); got != "external" {
		t.Fatalf("unexpected controller: %q", got)
	}
}

func TestBuildRuntimeController_EmbeddedModeRequiresBinary(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := buildRuntimeController(HubConfig{
		HubConfig: config.HubConfig{
			PublicKey:  "public",
			PrivateKey: "private",
			ConfigPath: "/tmp/xray-hub.json",
		},
		RuntimeMode: config.XrayRuntimeEmbedded,
	}, logger)
	if err == nil {
		t.Fatal("expected error for missing xray binary in embedded mode")
	}
}
