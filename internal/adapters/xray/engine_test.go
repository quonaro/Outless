package xray

import (
	"encoding/json"
	"testing"
)

func TestParseGRPCTarget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"http://localhost:10085", "localhost:10085"},
		{"http://127.0.0.1:10085", "127.0.0.1:10085"},
		{"https://xray:443", "xray:443"},
	}
	for _, tc := range cases {
		got, err := parseGRPCTarget(tc.in)
		if err != nil {
			t.Fatalf("parseGRPCTarget(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseGRPCTarget(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildProbeRoutingConfig(t *testing.T) {
	t.Parallel()
	cfg, err := buildProbeRoutingConfig("t_rule", "socks-in", "www.example.com", "out_t")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("nil config")
	}
}

func TestParseVLESSURLRealitySpx(t *testing.T) {
	t.Parallel()
	raw := "vless://b7128fbf-7b20-479d-9c18-82edb96cca5c@82.22.41.75:51855?type=tcp&security=reality&sni=www.yandex.ru&fp=firefox&pbk=testpbk&sid=e1e386&spx=%2Fwp#r"
	p, err := parseVLESSURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spx != "/wp" {
		t.Fatalf("spx: got %q want /wp", p.Spx)
	}
	ss := p.streamSettings()
	rs, ok := ss["realitySettings"].(map[string]any)
	if !ok {
		t.Fatal("missing realitySettings")
	}
	if rs["spiderX"] != "/wp" {
		t.Fatalf("spiderX: got %v", rs["spiderX"])
	}
}

func TestVerifyRoutingRuleJSON(t *testing.T) {
	t.Parallel()
	rule := map[string]any{
		"ruleTag":     "x",
		"type":        "field",
		"inboundTag":  []string{"socks-in"},
		"domain":      []string{"full:www.google.com"},
		"outboundTag": "direct",
	}
	b, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyRoutingRuleJSON(b); err != nil {
		t.Fatal(err)
	}
}
