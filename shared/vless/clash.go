package vless

import (
	"fmt"
	"net/url"
	"strconv"
)

// FromClash converts a Clash/ClashMeta proxy dictionary into a vless:// URL.
func FromClash(m map[string]any) (string, error) {
	if getString(m, "type") != scheme {
		return "", fmt.Errorf("not a vless proxy")
	}

	server := getString(m, "server")
	port := getInt(m, "port")
	uuid := getString(m, "uuid")

	if server == "" || port == 0 || uuid == "" {
		return "", fmt.Errorf("missing server, port or uuid")
	}

	q := url.Values{}

	network := valueOr(getString(m, "network"), "tcp")
	q.Set("type", network)

	security := "none"
	if isTrue(m["tls"]) {
		security = "tls"
	}
	if _, ok := m["reality-opts"]; ok {
		security = "reality"
	}
	q.Set("security", security)

	if sni := getString(m, "servername"); sni != "" {
		q.Set("sni", sni)
	}
	if fp := getString(m, "client-fingerprint"); fp != "" {
		q.Set("fp", fp)
	}
	if flow := getString(m, "flow"); flow != "" {
		q.Set("flow", flow)
	}

	setNetworkOptions(q, m, network)
	setRealityOptions(q, m)

	u := url.URL{
		Scheme:   scheme,
		User:     url.User(uuid),
		Host:     fmt.Sprintf("%s:%d", server, port),
		RawQuery: q.Encode(),
		Fragment: getString(m, "name"),
	}

	return u.String(), nil
}

func setNetworkOptions(q url.Values, m map[string]any, network string) {
	if network == "grpc" {
		setGRPCOptions(q, m)
		return
	}
	setPathHostOptions(q, m, network+"-opts")
}

func setPathHostOptions(q url.Values, m map[string]any, optsKey string) {
	opts, ok := getMap(m, optsKey)
	if !ok {
		return
	}
	if p := getString(opts, "path"); p != "" {
		q.Set("path", p)
	}
	if headers, ok := getMap(opts, "headers"); ok {
		if h := firstString(headers, "Host"); h != "" {
			q.Set("host", h)
		}
	}
}

func setGRPCOptions(q url.Values, m map[string]any) {
	opts, ok := getMap(m, "grpc-opts")
	if !ok {
		return
	}
	if s := getString(opts, "grpc-service-name"); s != "" {
		q.Set("serviceName", s)
	}
}

func setRealityOptions(q url.Values, m map[string]any) {
	if ro, ok := getMap(m, "reality-opts"); ok {
		if pk := getString(ro, "public-key"); pk != "" {
			q.Set("pbk", pk)
		}
		if sid := getString(ro, "short-id"); sid != "" {
			q.Set("sid", sid)
		}
		if spx := getString(ro, "spider-x"); spx != "" {
			q.Set("spx", spx)
		}
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func firstString(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func getMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key].(map[string]any)
	return v, ok
}

func isTrue(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
