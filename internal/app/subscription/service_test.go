package subscription

import "testing"

func TestExtractNodeHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "extract ip host from vless url",
			in:   "vless://uuid@1.2.3.4:443?security=reality#name",
			want: "1.2.3.4",
		},
		{
			name: "extract domain host from vless url",
			in:   "vless://uuid@node.example.com:443?security=reality#name",
			want: "node.example.com",
		},
		{
			name: "fallback for invalid url",
			in:   "://bad",
			want: "unknown-host",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractNodeHost(tt.in)
			if got != tt.want {
				t.Fatalf("extractNodeHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildConnectionRemark(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   string
		host    string
		country string
		want    string
	}{
		{
			name:    "builds standard remark",
			group:   "test",
			host:    "1.2.3.4",
			country: "DE",
			want:    "test-1.2.3.4-DE",
		},
		{
			name:    "replaces spaces and slashes",
			group:   "group one",
			host:    "node/domain",
			country: "US",
			want:    "group_one-node_domain-US",
		},
		{
			name:    "uses fallback values",
			group:   "",
			host:    "",
			country: "",
			want:    "ungrouped-unknown-host-XX",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildConnectionRemark(tt.group, tt.host, tt.country)
			if got != tt.want {
				t.Fatalf("buildConnectionRemark() = %q, want %q", got, tt.want)
			}
		})
	}
}
