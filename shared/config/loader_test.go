package config

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreate_NewConfigStructure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "outless.yaml")
	yamlContent := `database:
  url: "postgres://example"
jwt:
  secret: "test-secret"
  expiry: "24h"
admin:
  login: "admin"
  password: "pass"
api:
  shutdown: "10s"
geoip:
  db_path: "/tmp/geo.mmdb"
  db_url: "https://example.com/geo.mmdb"
  auto: true
  ttl: "12h"
router:
  port: 443
  sni: "www.google.com"
  public_key: "public"
  private_key: "private"
  short_id: "abc"
  fingerprint: "chrome"
  address: ":443"
  sync_interval: "5s"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	loader := NewLoader(slog.New(slog.NewTextHandler(io.Discard, nil)))
	cfg := DefaultConfig()
	if err := loader.LoadOrCreate(path, &cfg); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := cfg.Database.URL; got != "postgres://example" {
		t.Fatalf("unexpected database url: %q", got)
	}
	if got := cfg.JWT.Secret; got != "test-secret" {
		t.Fatalf("unexpected jwt secret: %q", got)
	}
	if got := cfg.GeoIP.TTL; got != 12*time.Hour {
		t.Fatalf("unexpected geoip ttl: %s", got)
	}
	if got := cfg.Router.Port; got != 443 {
		t.Fatalf("unexpected router port: %d", got)
	}
}
