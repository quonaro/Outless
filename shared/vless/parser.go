package vless

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Parsed holds the extracted parts of a vless:// URL.
type Parsed struct {
	Host       string
	Port       int
	UUID       string
	Encryption string
	Flow       string
	Network    string
	Security   string
	SNI        string
	FP         string
	PBK        string
	SID        string
	ALPN       []string
	Path       string
	HostHeader string
	Service    string
	SPX        string
	Name       string
}

// ParseURL parses a vless://uuid@host:port?params#remark URL into its parts.
func ParseURL(raw string) (Parsed, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Parsed{}, fmt.Errorf("parsing vless url: %w", err)
	}
	if u.Scheme != "vless" {
		return Parsed{}, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if u.User == nil {
		return Parsed{}, fmt.Errorf("vless url missing user")
	}

	host := u.Hostname()
	portStr := u.Port()
	if portStr == "" {
		portStr = "443"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Parsed{}, fmt.Errorf("parsing port: %w", err)
	}

	q := u.Query()
	p := Parsed{
		Host:       host,
		Port:       port,
		UUID:       u.User.Username(),
		Encryption: valueOr(q.Get("encryption"), "none"),
		Flow:       q.Get("flow"),
		Network:    valueOr(q.Get("type"), "tcp"),
		Security:   valueOr(q.Get("security"), "none"),
		SNI:        q.Get("sni"),
		FP:         q.Get("fp"),
		PBK:        q.Get("pbk"),
		SID:        q.Get("sid"),
		Path:       q.Get("path"),
		HostHeader: q.Get("host"),
		Service:    q.Get("serviceName"),
		SPX:        strings.TrimSpace(q.Get("spx")),
		Name:       strings.TrimSpace(u.Fragment),
	}
	if alpn := q.Get("alpn"); alpn != "" {
		p.ALPN = strings.Split(alpn, ",")
	}
	return p, nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ExtractIPFromVLESS extracts the host (IP or domain) from a vless:// URL.
func ExtractIPFromVLESS(raw string) string {
	parsed, err := ParseURL(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}
