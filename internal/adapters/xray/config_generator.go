package xray

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"outless/internal/domain"
)

// HubInboundConfig holds Reality inbound parameters for the generated Xray config.
type HubInboundConfig struct {
	Listen      string
	Port        int
	SNI         string
	PrivateKey  string
	ShortID     string
	Destination string
}

// GenerateHubConfig builds a full Xray config for the Hub relay.
//
// The resulting config:
//   - exposes a single VLESS Reality inbound on Port, trusting every active token UUID as a user;
//   - creates one VLESS outbound per healthy exit node (tagged by node.ID);
//   - uses per-user (balancer) routing so users of group G get load-balanced across G's exits;
//   - adds "direct" and "block" fallbacks for control traffic.
func GenerateHubConfig(tokens []domain.Token, nodes []domain.Node, inbound HubInboundConfig) ([]byte, error) {
	clients, userByGroup := buildClients(tokens)
	outbounds, nodesByGroup := buildOutbounds(nodes)
	balancers, routingRules := buildRouting(userByGroup, nodesByGroup)

	dest := inbound.Destination
	if dest == "" {
		dest = "www.google.com:443"
	}
	sni := inbound.SNI
	if sni == "" {
		sni = "www.google.com"
	}
	shortIDs := []string{""}
	if inbound.ShortID != "" {
		shortIDs = []string{inbound.ShortID}
	}
	listen := inbound.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	port := inbound.Port
	if port == 0 {
		port = 443
	}

	cfg := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "vless-in",
				"listen":   listen,
				"port":     port,
				"protocol": "vless",
				"settings": map[string]any{
					"clients":    clients,
					"decryption": "none",
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "reality",
					"realitySettings": map[string]any{
						"show":        false,
						"dest":        dest,
						"xver":        0,
						"serverNames": []string{sni},
						"privateKey":  inbound.PrivateKey,
						"shortIds":    shortIDs,
					},
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls"},
				},
			},
		},
		"outbounds": append(outbounds,
			map[string]any{"tag": "direct", "protocol": "freedom"},
			map[string]any{"tag": "block", "protocol": "blackhole"},
		),
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"balancers":      balancers,
			"rules":          routingRules,
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling xray config: %w", err)
	}

	return data, nil
}

// buildClients returns VLESS client entries for the inbound plus a map groupID -> []email
// used later to wire routing rules per group.
func buildClients(tokens []domain.Token) ([]map[string]any, map[string][]string) {
	clients := make([]map[string]any, 0, len(tokens))
	userByGroup := make(map[string][]string, 8)

	for _, token := range tokens {
		if token.UUID == "" {
			continue
		}

		email := fmt.Sprintf("token-%s@outless", token.ID)
		clients = append(clients, map[string]any{
			"id":    token.UUID,
			"email": email,
			"flow":  "xtls-rprx-vision",
			"level": 0,
		})

		if token.GroupID != "" {
			userByGroup[token.GroupID] = append(userByGroup[token.GroupID], email)
		}
	}

	return clients, userByGroup
}

// buildOutbounds creates one VLESS outbound per healthy exit node and returns
// nodes indexed by group id for routing resolution.
func buildOutbounds(nodes []domain.Node) ([]any, map[string][]domain.Node) {
	outbounds := make([]any, 0, len(nodes))
	nodesByGroup := make(map[string][]domain.Node, 8)

	for _, node := range nodes {
		if node.Status != domain.NodeStatusHealthy {
			continue
		}

		parsed, err := parseVLESSURL(node.URL)
		if err != nil {
			continue
		}

		tag := outboundTag(node.ID)
		outbounds = append(outbounds, map[string]any{
			"tag":      tag,
			"protocol": "vless",
			"settings": map[string]any{
				"vnext": []any{
					map[string]any{
						"address": parsed.host,
						"port":    parsed.port,
						"users": []any{
							map[string]any{
								"id":         parsed.uuid,
								"encryption": parsed.encryption,
								"flow":       parsed.flow,
							},
						},
					},
				},
			},
			"streamSettings": parsed.streamSettings(),
		})

		nodesByGroup[node.GroupID] = append(nodesByGroup[node.GroupID], node)
	}

	return outbounds, nodesByGroup
}

// buildRouting creates a balancer per group plus matching routing rules that
// route users of that group to their balancer. Groups without any healthy node
// are sent to "block" to avoid leaking traffic through wrong outbound.
func buildRouting(userByGroup map[string][]string, nodesByGroup map[string][]domain.Node) ([]any, []any) {
	balancers := make([]any, 0, len(userByGroup))
	rules := make([]any, 0, len(userByGroup))

	for groupID, emails := range userByGroup {
		nodes := nodesByGroup[groupID]

		if len(nodes) == 0 {
			rules = append(rules, map[string]any{
				"type":        "field",
				"inboundTag":  []string{"vless-in"},
				"user":        emails,
				"outboundTag": "block",
			})
			continue
		}

		balancerTag := balancerTagFor(groupID)
		selectors := make([]string, 0, len(nodes))
		for _, node := range nodes {
			selectors = append(selectors, outboundTag(node.ID))
		}

		balancers = append(balancers, map[string]any{
			"tag":      balancerTag,
			"selector": selectors,
			"strategy": map[string]any{"type": "random"},
		})

		rules = append(rules, map[string]any{
			"type":         "field",
			"inboundTag":   []string{"vless-in"},
			"user":         emails,
			"balancerTag":  balancerTag,
		})
	}

	return balancers, rules
}

func outboundTag(nodeID string) string {
	return "out-" + sanitizeTag(nodeID)
}

func balancerTagFor(groupID string) string {
	return "bal-" + sanitizeTag(groupID)
}

func sanitizeTag(raw string) string {
	b := strings.Builder{}
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// parsedVLESS holds the extracted parts of a vless:// URL required for an Xray outbound.
type parsedVLESS struct {
	host       string
	port       int
	uuid       string
	encryption string
	flow       string
	network    string
	security   string
	sni        string
	fp         string
	pbk        string
	sid        string
	alpn       []string
	path       string
	hostHeader string
	serviceName string
}

// parseVLESSURL parses a vless://uuid@host:port?params#remark URL into its parts.
func parseVLESSURL(raw string) (parsedVLESS, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return parsedVLESS{}, fmt.Errorf("parsing vless url: %w", err)
	}
	if u.Scheme != "vless" {
		return parsedVLESS{}, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if u.User == nil {
		return parsedVLESS{}, fmt.Errorf("vless url missing user")
	}

	host := u.Hostname()
	portStr := u.Port()
	if portStr == "" {
		portStr = "443"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return parsedVLESS{}, fmt.Errorf("parsing port: %w", err)
	}

	q := u.Query()
	p := parsedVLESS{
		host:        host,
		port:        port,
		uuid:        u.User.Username(),
		encryption:  valueOr(q.Get("encryption"), "none"),
		flow:        q.Get("flow"),
		network:     valueOr(q.Get("type"), "tcp"),
		security:    valueOr(q.Get("security"), "none"),
		sni:         q.Get("sni"),
		fp:          q.Get("fp"),
		pbk:         q.Get("pbk"),
		sid:         q.Get("sid"),
		path:        q.Get("path"),
		hostHeader:  q.Get("host"),
		serviceName: q.Get("serviceName"),
	}
	if alpn := q.Get("alpn"); alpn != "" {
		p.alpn = strings.Split(alpn, ",")
	}
	return p, nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// streamSettings translates the parsed VLESS URL into Xray streamSettings.
func (p parsedVLESS) streamSettings() map[string]any {
	stream := map[string]any{
		"network":  p.network,
		"security": p.security,
	}

	switch p.security {
	case "reality":
		reality := map[string]any{
			"show":        false,
			"fingerprint": valueOr(p.fp, "chrome"),
		}
		if p.sni != "" {
			reality["serverName"] = p.sni
		}
		if p.pbk != "" {
			reality["publicKey"] = p.pbk
		}
		if p.sid != "" {
			reality["shortId"] = p.sid
		}
		stream["realitySettings"] = reality
	case "tls":
		tls := map[string]any{
			"fingerprint": valueOr(p.fp, "chrome"),
		}
		if p.sni != "" {
			tls["serverName"] = p.sni
		}
		if len(p.alpn) > 0 {
			tls["alpn"] = p.alpn
		}
		stream["tlsSettings"] = tls
	}

	switch p.network {
	case "ws":
		ws := map[string]any{"path": valueOr(p.path, "/")}
		if p.hostHeader != "" {
			ws["headers"] = map[string]string{"Host": p.hostHeader}
		}
		stream["wsSettings"] = ws
	case "grpc":
		stream["grpcSettings"] = map[string]any{"serviceName": p.serviceName}
	}

	return stream
}
