package domain

import "strings"

// NormalizeCountryCode uppercases a two-letter ISO 3166-1 alpha-2 code; otherwise returns trimmed input unchanged (legacy rows).
func NormalizeCountryCode(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 2 {
		return s
	}
	u := strings.ToUpper(s)
	if u[0] >= 'A' && u[0] <= 'Z' && u[1] >= 'A' && u[1] <= 'Z' {
		return u
	}
	return s
}
