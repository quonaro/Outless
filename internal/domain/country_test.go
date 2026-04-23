package domain

import "testing"

func TestNormalizeCountryCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"us", "US"},
		{"DE", "DE"},
		{"  fr ", "FR"},
		{"", ""},
		{"USA", "USA"},
		{"1A", "1A"},
	}
	for _, tc := range cases {
		if got := NormalizeCountryCode(tc.in); got != tc.want {
			t.Errorf("NormalizeCountryCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
