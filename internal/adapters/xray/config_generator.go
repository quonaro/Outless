package xray

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
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
//   - uses direct routing by email so each token+node combo routes to its specific outbound;
//   - adds "direct" and "block" fallbacks for control traffic.
func GenerateHubConfig(tokens []domain.Token, nodes []domain.Node, inbound HubInboundConfig, logger *slog.Logger) ([]byte, error) {
	logger.Debug("Generating Xray Hub config",
		slog.Int("tokens", len(tokens)),
		slog.Int("nodes", len(nodes)),
		slog.String("inbound_dest", inbound.Destination),
		slog.String("inbound_sni", inbound.SNI),
		slog.String("inbound_shortid", inbound.ShortID),
	)
	clients := buildClients(tokens, nodes, logger)
	outbounds, _, _ := buildOutbounds(nodes, logger)
	routingRules := buildDirectRouting(clients, logger)

	dest := inbound.Destination
	sni := inbound.SNI
	// Use empty array when ShortID not set to match client that sends sid=""
	shortIDs := []string{}
	if inbound.ShortID != "" {
		shortIDs = []string{inbound.ShortID}
	}
	logger.Debug("REALITY inbound settings",
		slog.String("dest", dest),
		slog.String("sni", sni),
		slog.String("shortIds", fmt.Sprintf("%v", shortIDs)),
		slog.Bool("has_private_key", inbound.PrivateKey != ""),
	)
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
			"loglevel": "info",
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
			"rules":          routingRules,
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling xray config: %w", err)
	}

	if logger != nil {
		logger.Debug("Generated Xray Hub config",
			slog.Int("tokens", len(tokens)),
			slog.Int("nodes", len(nodes)),
			slog.Int("clients", len(clients)),
			slog.Int("outbounds", len(outbounds)),
			slog.Int("routing_rules", len(routingRules)),
		)
	}

	return data, nil
}

// generateUUIDFromTokenNode generates a deterministic UUID from tokenID and nodeID.
// Uses MD5 hash formatted as UUID.
func generateUUIDFromTokenNode(tokenID, nodeID string) string {
	if tokenID == "" || nodeID == "" {
		return ""
	}
	h := md5.New()
	h.Write([]byte(tokenID))
	h.Write([]byte(nodeID))
	hash := hex.EncodeToString(h.Sum(nil))
	// Format as UUID: 8-4-4-4-12
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hash[0:8], hash[8:12], hash[12:16], hash[16:20], hash[20:32])
}

// buildClients returns VLESS client entries for the inbound.
// Creates one client entry per token+node combination with unique UUID for direct routing.
func buildClients(tokens []domain.Token, nodes []domain.Node, logger *slog.Logger) []map[string]any {
	clients := make([]map[string]any, 0)

	for _, token := range tokens {
		if token.UUID == "" {
			continue
		}

		// Get allowed groups for this token
		allowedGroups := make(map[string]struct{})
		for _, groupID := range token.GroupIDs {
			allowedGroups[groupID] = struct{}{}
		}
		if len(allowedGroups) == 0 && token.GroupID != "" {
			allowedGroups[token.GroupID] = struct{}{}
		}
		allGroupsAllowed := len(allowedGroups) == 0

		if logger != nil {
			logger.Debug(fmt.Sprintf("Processing token: id=%s uuid=%s all_groups=%v", token.ID, token.UUID, allGroupsAllowed))
		}

		hasAccess := false

		// Create one client entry per accessible node with unique UUID
		for _, node := range nodes {
			if node.Status != domain.NodeStatusHealthy {
				continue
			}

			if !allGroupsAllowed {
				if _, ok := allowedGroups[node.GroupID]; !ok {
					continue
				}
			}

			// Generate unique UUID for this token+node combination
			uuid := generateUUIDFromTokenNode(token.ID, node.ID)
			email := fmt.Sprintf("token-%s-node-%s@outless", token.ID, node.ID)
			clients = append(clients, map[string]any{
				"id":    uuid,
				"email": email,
				"flow":  "xtls-rprx-vision",
				"level": 0,
			})

			if logger != nil {
				logger.Debug("Created client entry",
					slog.String("token_id", token.ID),
					slog.String("node_id", node.ID),
					slog.String("group_id", node.GroupID),
					slog.String("uuid", uuid),
					slog.String("email", email),
				)
			}

			hasAccess = true
		}

		// If token has no access to any nodes, create a blocked entry
		if !hasAccess {
			email := fmt.Sprintf("token-%s@outless", token.ID)
			clients = append(clients, map[string]any{
				"id":    token.UUID,
				"email": email,
				"flow":  "xtls-rprx-vision",
				"level": 0,
			})

			if logger != nil {
				logger.Warn("Token has no access to any nodes, creating blocked entry",
					slog.String("token_id", token.ID),
					slog.String("email", email),
				)
			}
		}
	}

	return clients
}

// buildDirectRouting creates direct routing rules by email without balancers.
// Each email (token-node) routes directly to its specific outbound.
func buildDirectRouting(clients []map[string]any, logger *slog.Logger) []any {
	rules := make([]any, 0)

	for _, client := range clients {
		email, ok := client["email"].(string)
		if !ok {
			continue
		}

		// Extract node ID from email (format: token-{tokenID}-node-{nodeID}@outless)
		// If email doesn't have node ID, it's a blocked entry
		if !strings.Contains(email, "-node-") {
			rules = append(rules, map[string]any{
				"type":        "field",
				"inboundTag":  []string{"vless-in"},
				"user":        []string{email},
				"outboundTag": "block",
			})

			if logger != nil {
				logger.Warn(fmt.Sprintf("Created block rule: email=%s", email))
			}
			continue
		}

		// Extract node ID from email
		parts := strings.Split(email, "-node-")
		if len(parts) < 2 {
			continue
		}
		nodeID := strings.TrimSuffix(parts[1], "@outless")
		outboundTag := outboundTag(nodeID)

		// Create direct routing rule
		rules = append(rules, map[string]any{
			"type":        "field",
			"inboundTag":  []string{"vless-in"},
			"user":        []string{email},
			"outboundTag": outboundTag,
		})

		if logger != nil {
			logger.Debug("Created routing rule",
				slog.String("email", email),
				slog.String("node_id", nodeID),
				slog.String("outbound", outboundTag),
			)
		}
	}

	return rules
}

// buildOutbounds creates one VLESS outbound per healthy exit node and returns
// nodes indexed by group id for routing resolution.
func buildOutbounds(nodes []domain.Node, logger *slog.Logger) ([]any, map[string][]domain.Node, []string) {
	outbounds := make([]any, 0, len(nodes))
	nodesByGroup := make(map[string][]domain.Node, 8)
	allSelectors := make([]string, 0, len(nodes))

	for _, node := range nodes {
		if node.Status != domain.NodeStatusHealthy {
			if logger != nil {
				logger.Debug(fmt.Sprintf("Skipping unhealthy node: id=%s status=%s", node.ID, node.Status))
			}
			continue
		}

		parsed, err := parseVLESSURL(node.URL)
		if err != nil {
			if logger != nil {
				logger.Error(fmt.Sprintf("Failed to parse VLESS URL: node=%s error=%s", node.ID, err.Error()))
			}
			continue
		}

		tag := outboundTag(node.ID)
		allSelectors = append(allSelectors, tag)
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

		if logger != nil {
			logger.Debug(fmt.Sprintf("Created outbound: node=%s group=%s tag=%s host=%s:%d", node.ID, node.GroupID, tag, parsed.host, parsed.port))
		}

		nodesByGroup[node.GroupID] = append(nodesByGroup[node.GroupID], node)
	}

	return outbounds, nodesByGroup, allSelectors
}

// buildRouting creates a balancer per group plus matching routing rules that
// route users of that group to their balancer. Groups without any healthy node
// are sent to "block" to avoid leaking traffic through wrong outbound.
func buildRouting(userByGroup map[string][]string, usersAnyGroup []string, nodesByGroup map[string][]domain.Node, allSelectors []string) ([]any, []any) {
	balancers := make([]any, 0, len(userByGroup)+1)
	rules := make([]any, 0, len(userByGroup)+1)

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
			"type":        "field",
			"inboundTag":  []string{"vless-in"},
			"user":        emails,
			"balancerTag": balancerTag,
		})
	}

	// Tokens without group_id get access to all healthy outbounds.
	if len(usersAnyGroup) > 0 {
		if len(allSelectors) == 0 {
			rules = append(rules, map[string]any{
				"type":        "field",
				"inboundTag":  []string{"vless-in"},
				"user":        usersAnyGroup,
				"outboundTag": "block",
			})
		} else {
			balancers = append(balancers, map[string]any{
				"tag":      "bal-all-groups",
				"selector": allSelectors,
				"strategy": map[string]any{"type": "random"},
			})
			rules = append(rules, map[string]any{
				"type":        "field",
				"inboundTag":  []string{"vless-in"},
				"user":        usersAnyGroup,
				"balancerTag": "bal-all-groups",
			})
		}
	}

	return balancers, rules
}

func tokenGroupIDs(token domain.Token) []string {
	seen := make(map[string]struct{}, len(token.GroupIDs))
	out := make([]string, 0, len(token.GroupIDs)+1)

	for _, groupID := range token.GroupIDs {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		out = append(out, groupID)
	}

	if token.GroupID != "" {
		if _, ok := seen[token.GroupID]; !ok {
			out = append(out, token.GroupID)
		}
	}

	return out
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
	host        string
	port        int
	uuid        string
	encryption  string
	flow        string
	network     string
	security    string
	sni         string
	fp          string
	pbk         string
	sid         string
	alpn        []string
	path        string
	hostHeader  string
	serviceName string
	// Spx is the Reality "spiderX" path from the sharing link (query key "spx").
	Spx string
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
		Spx:         strings.TrimSpace(q.Get("spx")),
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
			"show":         false,
			"fingerprint":  valueOr(p.fp, "chrome"),
			"masterKeyLog": "", // Disable master key logging to avoid file open errors
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
		if p.Spx != "" {
			reality["spiderX"] = p.Spx
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
