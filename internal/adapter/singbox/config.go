package singbox

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"

	"outless/internal/domain"
	"outless/internal/utils"
	"outless/shared/vless"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

// HubInboundConfig holds REALITY inbound parameters for the generated sing-box config.
type HubInboundConfig struct {
	Listen     string
	Port       int
	SNI        string
	Handshake  string
	PrivateKey string
	ShortID    string
	LogLevel   string
}

const (
	tagInbound = "vless-in"
	tagBlock   = "block"
	flowVision = "xtls-rprx-vision"
)

// userName builds a deterministic sing-box inbound user name for a token+node pair.
func userName(tokenID, nodeID string) string {
	return fmt.Sprintf("t-%s-n-%s", tokenID, nodeID)
}

func outboundTag(nodeID string) string {
	return "out-" + sanitizeTag(nodeID)
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

// GenerateOptions builds a full sing-box option.Options for the hub relay.
//
// The result exposes one VLESS REALITY inbound per HubInboundConfig, all sharing
// one user per token+node combination, one VLESS outbound per exit node, and
// route rules that send each user to its specific outbound. Unmatched traffic
// is blocked.
func GenerateOptions(
	tokens []domain.Token,
	nodes []domain.Node,
	inbounds []HubInboundConfig,
	singboxLogLevel string,
	logger *slog.Logger,
) (option.Options, error) {
	users, rules, err := buildUsersAndRules(tokens, nodes, logger)
	if err != nil {
		return option.Options{}, err
	}

	outbounds, err := buildOutbounds(nodes, logger)
	if err != nil {
		return option.Options{}, err
	}
	outbounds = append(outbounds,
		option.Outbound{Type: C.TypeBlock, Tag: tagBlock},
	)

	inboundOptions, err := buildInbounds(inbounds, users, logger)
	if err != nil {
		return option.Options{}, err
	}

	logLevel := strings.TrimSpace(singboxLogLevel)
	if logLevel == "" {
		logLevel = "warn"
	}

	opts := option.Options{
		Log:       &option.LogOptions{Level: logLevel, Timestamp: false},
		Inbounds:  inboundOptions,
		Outbounds: outbounds,
		Route: &option.RouteOptions{
			Rules: rules,
			Final: tagBlock,
		},
	}

	if logger != nil {
		logger.Debug("generated sing-box options",
			slog.Int("tokens", len(tokens)),
			slog.Int("nodes", len(nodes)),
			slog.Int("inbounds", len(inboundOptions)),
			slog.Int("users", len(users)),
			slog.Int("outbounds", len(outbounds)),
			slog.Int("rules", len(rules)),
		)
	}

	return opts, nil
}

func buildInbounds(inbounds []HubInboundConfig, users []option.VLESSUser, logger *slog.Logger) ([]option.Inbound, error) {
	result := make([]option.Inbound, 0, len(inbounds))
	for i, inbound := range inbounds {
		listen := inbound.Listen
		if listen == "" {
			listen = "0.0.0.0"
		}
		listenAddr, err := netip.ParseAddr(listen)
		if err != nil {
			return nil, fmt.Errorf("parsing listen address %q: %w", listen, err)
		}

		port := inbound.Port
		if port == 0 {
			port = 443
		}

		handshake := inbound.Handshake
		if handshake == "" {
			handshake = inbound.SNI
		}
		if handshake == "" {
			handshake = "www.google.com"
		}

		sni := inbound.SNI
		if sni == "" {
			sni = handshake
		}

		shortID := inbound.ShortID
		if shortID == "" {
			shortID = "0000000000000000"
		}
		shortIDs := option.Listable[string]{shortID}

		reality := &option.InboundRealityOptions{
			Enabled:    true,
			PrivateKey: inbound.PrivateKey,
			ShortID:    shortIDs,
			Handshake: option.InboundRealityHandshakeOptions{
				ServerOptions: option.ServerOptions{Server: handshake, ServerPort: 443},
			},
		}

		vlessInbound := option.VLESSInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     option.NewListenAddress(listenAddr),
				ListenPort: uint16(port),
			},
			Users: users,
			InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
				TLS: &option.InboundTLSOptions{
					Enabled:    true,
					ServerName: sni,
					Reality:    reality,
				},
			},
		}

		tag := fmt.Sprintf("%s-%d", tagInbound, i)
		result = append(result, option.Inbound{Type: C.TypeVLESS, Tag: tag, VLESSOptions: vlessInbound})
	}
	return result, nil
}

// buildUsersAndRules creates one inbound user per accessible token+node pair and
// matching auth_user route rules. Tokens without node access get a blocked user.
func buildUsersAndRules(tokens []domain.Token, nodes []domain.Node, logger *slog.Logger) ([]option.VLESSUser, []option.Rule, error) {
	users := make([]option.VLESSUser, 0)
	rules := make([]option.Rule, 0)

	for _, token := range tokens {
		if token.UUID == "" {
			continue
		}

		allowed := make(map[string]struct{})
		for _, gid := range token.GroupIDs {
			allowed[gid] = struct{}{}
		}
		if len(allowed) == 0 && token.GroupID != "" {
			allowed[token.GroupID] = struct{}{}
		}
		allGroups := len(allowed) == 0

		hasAccess := false
		for _, node := range nodes {
			if !allGroups {
				if _, ok := allowed[node.GroupID]; !ok {
					continue
				}
			}
			name := userName(token.ID, node.ID)
			uuid := utils.GenerateUUIDFromTokenNode(token.ID, node.ID)
			users = append(users, option.VLESSUser{Name: name, UUID: uuid, Flow: flowVision})
			rules = append(rules, routeUserTo(name, outboundTag(node.ID)))
			hasAccess = true
		}

		if !hasAccess {
			name := fmt.Sprintf("t-%s-blocked", token.ID)
			users = append(users, option.VLESSUser{Name: name, UUID: token.UUID, Flow: flowVision})
			rules = append(rules, routeUserTo(name, tagBlock))
		}
	}

	return users, rules, nil
}

func routeUserTo(authUser, outbound string) option.Rule {
	return option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			AuthUser: option.Listable[string]{authUser},
			Outbound: outbound,
		},
	}
}

// buildOutbounds creates one outbound per exit node.
// Self-nodes use a direct outbound; others use VLESS.
func buildOutbounds(nodes []domain.Node, logger *slog.Logger) ([]option.Outbound, error) {
	outbounds := make([]option.Outbound, 0, len(nodes))

	for _, node := range nodes {
		if node.IsSelf {
			outbounds = append(outbounds, option.Outbound{
				Type: C.TypeDirect,
				Tag:  outboundTag(node.ID),
			})
			continue
		}

		parsed, err := vless.ParseURL(node.URL)
		if err != nil {
			if logger != nil {
				logger.Error("failed to parse VLESS URL", slog.String("node", node.ID), slog.String("error", err.Error()))
			}
			continue
		}

		vlessOut := option.VLESSOutboundOptions{
			ServerOptions: option.ServerOptions{Server: parsed.Host, ServerPort: uint16(parsed.Port)},
			UUID:          parsed.UUID,
			Flow:          parsed.Flow,
		}
		if tls := buildOutboundTLS(parsed); tls != nil {
			vlessOut.TLS = tls
		}
		if transport := buildTransport(parsed); transport != nil {
			vlessOut.Transport = transport
		}

		outbounds = append(outbounds, option.Outbound{
			Type:         C.TypeVLESS,
			Tag:          outboundTag(node.ID),
			VLESSOptions: vlessOut,
		})
	}

	return outbounds, nil
}

func buildOutboundTLS(p vless.Parsed) *option.OutboundTLSOptions {
	switch p.Security {
	case "reality":
		tls := &option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: p.SNI,
			Reality:    &option.OutboundRealityOptions{Enabled: true, PublicKey: p.PBK, ShortID: p.SID},
		}
		fp := p.FP
		if fp == "" {
			fp = "chrome"
		}
		tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: fp}
		return tls
	case "tls":
		tls := &option.OutboundTLSOptions{Enabled: true, ServerName: p.SNI}
		if len(p.ALPN) > 0 {
			tls.ALPN = p.ALPN
		}
		if p.FP != "" {
			tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: p.FP}
		}
		return tls
	default:
		return nil
	}
}

func buildTransport(p vless.Parsed) *option.V2RayTransportOptions {
	switch p.Network {
	case "ws":
		path := p.Path
		if path == "" {
			path = "/"
		}
		ws := option.V2RayWebsocketOptions{Path: path}
		if p.HostHeader != "" {
			ws.Headers = option.HTTPHeader{"Host": option.Listable[string]{p.HostHeader}}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: ws}
	case "grpc":
		return &option.V2RayTransportOptions{
			Type:        C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{ServiceName: p.Service},
		}
	default:
		return nil
	}
}
