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

// StreamSettings translates the parsed VLESS URL into Xray streamSettings map.
func (p Parsed) StreamSettings() map[string]any {
	stream := map[string]any{
		"network":  p.Network,
		"security": p.Security,
	}

	switch p.Security {
	case "reality":
		reality := map[string]any{
			"show":        false,
			"fingerprint": valueOr(p.FP, "chrome"),
		}
		if p.SNI != "" {
			reality["serverName"] = p.SNI
		}
		if p.PBK != "" {
			reality["publicKey"] = p.PBK
		}
		if p.SID != "" {
			reality["shortId"] = p.SID
		}
		if p.SPX != "" {
			reality["spiderX"] = p.SPX
		}
		stream["realitySettings"] = reality
	case "tls":
		tls := map[string]any{
			"fingerprint": valueOr(p.FP, "chrome"),
		}
		if p.SNI != "" {
			tls["serverName"] = p.SNI
		}
		if len(p.ALPN) > 0 {
			tls["alpn"] = p.ALPN
		}
		stream["tlsSettings"] = tls
	}

	switch p.Network {
	case "ws":
		ws := map[string]any{"path": valueOr(p.Path, "/")}
		if p.HostHeader != "" {
			ws["headers"] = map[string]string{"Host": p.HostHeader}
		}
		stream["wsSettings"] = ws
	case "grpc":
		stream["grpcSettings"] = map[string]any{"serviceName": p.Service}
	case "http", "httpupgrade", "splithttp", "xhttp":
		// HTTP-based transports use minimal config
		if p.Path != "" {
			stream["httpSettings"] = map[string]any{"path": p.Path}
		}
		if p.HostHeader != "" {
			if stream["httpSettings"] == nil {
				stream["httpSettings"] = map[string]any{}
			}
			httpSettings := stream["httpSettings"].(map[string]any)
			httpSettings["host"] = []string{p.HostHeader}
		}
	}

	return stream
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ExtractIPFromVLESS extracts the IP address from a vless:// URL.
// Returns empty string if the host is not an IP address.
func ExtractIPFromVLESS(raw string) string {
	parsed, err := ParseURL(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}
