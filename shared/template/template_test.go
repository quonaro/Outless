package template

import (
	"testing"
	"time"
)

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		data     TemplateData
		expected string
	}{
		{
			name:     "simple variable",
			tmpl:     "{{vless.country}}",
			data:     TemplateData{VLESS: VLESSData{Country: "Poland"}},
			expected: "Poland",
		},
		{
			name: "multiple variables",
			tmpl: "{{vless.country}} | {{vless.group}}",
			data: TemplateData{
				VLESS: VLESSData{
					Country: "Poland",
					Group:   "premium",
				},
			},
			expected: "Poland | premium",
		},
		{
			name:     "nested variable",
			tmpl:     "{{vless.host}}",
			data:     TemplateData{VLESS: VLESSData{Host: "example.com"}},
			expected: "example.com",
		},
		{
			name:     "fallback to string literal",
			tmpl:     "{{vless.name|\"Unknown\"}}",
			data:     TemplateData{VLESS: VLESSData{Name: ""}},
			expected: "Unknown",
		},
		{
			name:     "fallback to variable",
			tmpl:     "{{vless.name|vless.host}}",
			data:     TemplateData{VLESS: VLESSData{Name: "", Host: "example.com"}},
			expected: "example.com",
		},
		{
			name:     "no fallback when value exists",
			tmpl:     "{{vless.name|\"Unknown\"}}",
			data:     TemplateData{VLESS: VLESSData{Name: "My Node"}},
			expected: "My Node",
		},
		{
			name:     "ping variable",
			tmpl:     "{{vless.ping}}ms",
			data:     TemplateData{VLESS: VLESSData{Ping: "150"}},
			expected: "150ms",
		},
		{
			name:     "user variable",
			tmpl:     "User: {{vless.user}}",
			data:     TemplateData{VLESS: VLESSData{User: "user@example.com"}},
			expected: "User: user@example.com",
		},
		{
			name:     "country flag",
			tmpl:     "{{vless.country_flag}} {{vless.country}}",
			data:     TemplateData{VLESS: VLESSData{CountryFlag: "🇵🇱", Country: "Poland"}},
			expected: "🇵🇱 Poland",
		},
		{
			name:     "empty variable returns original",
			tmpl:     "{{vless.name}}",
			data:     TemplateData{VLESS: VLESSData{Name: ""}},
			expected: "{{vless.name}}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RenderTemplate(tc.tmpl, tc.data)
			if result != tc.expected {
				t.Errorf("RenderTemplate() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestBuildTemplateData(t *testing.T) {
	vlessData := VLESSData{
		Name:       "Poland Node",
		Host:       "example.com",
		Port:       443,
		SNI:        "example.com",
		Security:   "reality",
		Encryption: "none",
		Flow:       "xtls-rprx-vision",
		FP:         "chrome",
	}

	result := BuildTemplateData(vlessData, "Poland", "PL", "premium", 150*time.Millisecond, "user@example.com")

	if result.VLESS.Name != "Poland Node" {
		t.Errorf("VLESS.Name = %q, want %q", result.VLESS.Name, "Poland Node")
	}
	if result.VLESS.Country != "Poland" {
		t.Errorf("VLESS.Country = %q, want %q", result.VLESS.Country, "Poland")
	}
	if result.VLESS.CountryShort != "PL" {
		t.Errorf("VLESS.CountryShort = %q, want %q", result.VLESS.CountryShort, "PL")
	}
	if result.VLESS.Ping != "150" {
		t.Errorf("VLESS.Ping = %q, want %q", result.VLESS.Ping, "150")
	}
	if result.VLESS.Group != "premium" {
		t.Errorf("VLESS.Group = %q, want %q", result.VLESS.Group, "premium")
	}
	if result.VLESS.User != "user@example.com" {
		t.Errorf("VLESS.User = %q, want %q", result.VLESS.User, "user@example.com")
	}
}

func TestCountryFlagEmoji(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"PL", "🇵🇱"},
		{"US", "🇺🇸"},
		{"DE", "🇩🇪"},
		{"GB", "🇬🇧"},
		{"", "🏳️"},
		{"A", "🏳️"},
		{"ABC", "🏳️"},
		{"12", "🏳️"},
		{"ab", "\U0001F1E6\U0001F1E7"},
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			result := countryFlagEmoji(tc.code)
			if result != tc.expected {
				t.Errorf("countryFlagEmoji(%q) = %q, want %q", tc.code, result, tc.expected)
			}
		})
	}
}
