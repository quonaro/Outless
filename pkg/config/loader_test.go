package config

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreate_AppliesLegacyXrayFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "outless.yaml")
	yamlContent := `database:
  url: "postgres://example"
checker:
  workers: 8
  xray:
    admin_url: "http://legacy-probe:10085"
    probe_url: "https://probe.example.com/generate_204"
    socks_addr: "127.0.0.1:2080"
    geoip_db_path: "/tmp/geo.mmdb"
    geoip_db_url: "https://example.com/geo.mmdb"
    geoip_auto: true
    geoip_ttl: "12h"
hub:
  host: "hub.example.com"
  port: 443
  public_key: "public"
  private_key: "private"
  config_path: "/var/lib/outless/legacy-hub.json"
  xray_binary: "/usr/local/bin/xray"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	loader := NewLoader(slog.New(slog.NewTextHandler(io.Discard, nil)))
	cfg := DefaultConfig()
	if err := loader.LoadOrCreate(path, &cfg); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := cfg.Xray.Edge.ConfigPath; got != "/var/lib/outless/legacy-hub.json" {
		t.Fatalf("unexpected edge config path: %q", got)
	}
	if got := cfg.Xray.Edge.XrayBinary; got != "/usr/local/bin/xray" {
		t.Fatalf("unexpected edge binary: %q", got)
	}
	if got := cfg.Xray.Probe.AdminURL; got != "http://legacy-probe:10085" {
		t.Fatalf("unexpected probe admin url: %q", got)
	}
	if got := cfg.Xray.Probe.SocksAddr; got != "127.0.0.1:2080" {
		t.Fatalf("unexpected probe socks addr: %q", got)
	}
	if got := cfg.Xray.Probe.GeoIPTTL; got != 12*time.Hour {
		t.Fatalf("unexpected probe ttl: %s", got)
	}
}
