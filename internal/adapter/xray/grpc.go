package xray

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"outless/internal/domain"
	vlesspkg "outless/shared/vless"

	proxyman "github.com/xtls/xray-core/app/proxyman"
	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	router "github.com/xtls/xray-core/app/router"
	routerCommand "github.com/xtls/xray-core/app/router/command"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/proxy/vless"
	vlessInbound "github.com/xtls/xray-core/proxy/vless/inbound"
	vlessOutbound "github.com/xtls/xray-core/proxy/vless/outbound"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCRuntimeController manages external Xray via gRPC API.
type GRPCRuntimeController struct {
	logger     *slog.Logger
	tokenRepo  domain.TokenRepository
	nodeRepo   domain.NodeRepository
	grpcTarget string
	inboundTag string

	// Reality inbound settings
	inboundListen string
	inboundPort   int
	inboundSNI    string
	privateKey    string
	shortID       string
	destination   string

	mu   sync.Mutex
	conn *grpc.ClientConn
	hs   proxymanCommand.HandlerServiceClient
	rs   routerCommand.RoutingServiceClient
}

// NewGRPCRuntimeController creates a gRPC-based runtime controller.
func NewGRPCRuntimeController(
	logger *slog.Logger,
	tokenRepo domain.TokenRepository,
	nodeRepo domain.NodeRepository,
	adminURL string,
	inboundTag string,
	hubConfig HubInboundConfig,
) *GRPCRuntimeController {
	if inboundTag == "" {
		inboundTag = "vless-in"
	}
	target, err := parseGRPCTarget(adminURL)
	if err != nil {
		logger.Warn("invalid xray admin_url for gRPC", slog.String("admin_url", adminURL), slog.String("error", err.Error()))
		target = ""
	}

	listen := hubConfig.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	port := hubConfig.Port
	if port == 0 {
		port = 443
	}

	return &GRPCRuntimeController{
		logger:        logger,
		tokenRepo:     tokenRepo,
		nodeRepo:      nodeRepo,
		grpcTarget:    target,
		inboundTag:    inboundTag,
		inboundListen: listen,
		inboundPort:   port,
		inboundSNI:    hubConfig.SNI,
		privateKey:    hubConfig.PrivateKey,
		shortID:       hubConfig.ShortID,
		destination:   hubConfig.Destination,
	}
}

func (r *GRPCRuntimeController) ensureConn(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn != nil {
		return nil
	}

	conn, err := grpc.NewClient(r.grpcTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("grpc new client %s: %w", r.grpcTarget, err)
	}

	r.conn = conn
	r.hs = proxymanCommand.NewHandlerServiceClient(conn)
	r.rs = routerCommand.NewRoutingServiceClient(conn)

	r.logger.Info("connected to xray gRPC api", slog.String("target", r.grpcTarget))
	return nil
}

// Start ensures the inbound exists in external Xray via gRPC API.
func (r *GRPCRuntimeController) Start(_ string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.ensureConn(ctx); err != nil {
		return fmt.Errorf("ensuring gRPC connection: %w", err)
	}

	if err := r.ensureInbound(ctx); err != nil {
		return fmt.Errorf("ensuring inbound: %w", err)
	}

	r.logger.Info("grpc runtime controller started (external xray)")
	return nil
}

// Stop closes the gRPC connection.
func (r *GRPCRuntimeController) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn != nil {
		r.conn.Close()
		r.conn = nil
		r.logger.Info("disconnected from xray gRPC api")
	}
}

// Reload syncs the current tokens and nodes to Xray via gRPC API.
func (r *GRPCRuntimeController) Reload(_ string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := r.ensureConn(ctx); err != nil {
		return fmt.Errorf("ensuring gRPC connection: %w", err)
	}

	now := time.Now().UTC()

	tokens, err := r.tokenRepo.ListActive(ctx, now)
	if err != nil {
		return fmt.Errorf("listing active tokens: %w", err)
	}

	nodes, err := r.nodeRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("listing nodes: %w", err)
	}

	r.logger.Info("syncing to xray via gRPC",
		slog.Int("tokens", len(tokens)),
		slog.Int("nodes", len(nodes)),
	)

	clients := r.buildClients(tokens, nodes)

	// Add outbounds for all nodes
	for _, node := range nodes {
		if err := r.addNodeOutbound(ctx, node); err != nil {
			r.logger.Warn("failed to add outbound for node",
				slog.String("node_id", node.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	// Add all clients to inbound
	for _, client := range clients {
		if err := r.addClientToInbound(ctx, client); err != nil {
			r.logger.Warn("failed to add client to inbound",
				slog.String("email", client.Email),
				slog.String("error", err.Error()),
			)
		}
	}

	// Add routing rules for each client
	for _, client := range clients {
		if err := r.addRoutingRule(ctx, client); err != nil {
			r.logger.Warn("failed to add routing rule",
				slog.String("email", client.Email),
				slog.String("error", err.Error()),
			)
		}
	}

	r.logger.Info("xray sync completed via gRPC",
		slog.Int("clients_synced", len(clients)),
		slog.Int("nodes_synced", len(nodes)),
	)

	return nil
}

// Description returns controller description.
func (r *GRPCRuntimeController) Description() string {
	return "grpc-xray-api"
}

// RemoveUser removes a client from the inbound by email.
func (r *GRPCRuntimeController) RemoveUser(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := r.hs.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
		Tag: r.inboundTag,
		Operation: serial.ToTypedMessage(&proxymanCommand.RemoveUserOperation{
			Email: email,
		}),
	})
	if err != nil {
		r.logger.Warn("failed to remove user from inbound",
			slog.String("email", email),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("remove user: %w", err)
	}

	r.logger.Debug("removed user from inbound", slog.String("email", email))
	return nil
}

// RemoveRulesForUser removes all routing rules for a specific user email.
func (r *GRPCRuntimeController) RemoveRulesForUser(email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Remove rule by ruleTag (ruleTag format is "rule-{email}")
	ruleTag := "rule-" + email
	_, err := r.rs.RemoveRule(ctx, &routerCommand.RemoveRuleRequest{
		RuleTag: ruleTag,
	})
	if err != nil {
		// Rule might not exist, which is fine
		if !isNotFoundError(err) {
			r.logger.Warn("failed to remove routing rule",
				slog.String("rule_tag", ruleTag),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("remove rule: %w", err)
		}
	}

	r.logger.Debug("removed routing rule for user", slog.String("email", email), slog.String("rule_tag", ruleTag))
	return nil
}

// ClientInfo holds client information for routing.
type ClientInfo struct {
	UUID    string
	Email   string
	NodeID  string
	TokenID string
	Flow    string
}

func (r *GRPCRuntimeController) buildClients(tokens []domain.Token, nodes []domain.Node) []ClientInfo {
	clients := make([]ClientInfo, 0)

	for _, token := range tokens {
		if token.UUID == "" {
			continue
		}

		allowedGroups := make(map[string]struct{})
		for _, groupID := range token.GroupIDs {
			allowedGroups[groupID] = struct{}{}
		}
		if len(allowedGroups) == 0 && token.GroupID != "" {
			allowedGroups[token.GroupID] = struct{}{}
		}
		allGroupsAllowed := len(allowedGroups) == 0

		hasAccess := false

		for _, node := range nodes {
			if !allGroupsAllowed {
				if _, ok := allowedGroups[node.GroupID]; !ok {
					continue
				}
			}

			clientUUID := generateClientUUID(token.ID, node.ID)
			email := fmt.Sprintf("token-%s-node-%s@outless", token.ID, node.ID)

			clients = append(clients, ClientInfo{
				UUID:    clientUUID,
				Email:   email,
				NodeID:  node.ID,
				TokenID: token.ID,
				Flow:    "xtls-rprx-vision",
			})

			hasAccess = true
		}

		if !hasAccess {
			email := fmt.Sprintf("token-%s@outless", token.ID)
			clients = append(clients, ClientInfo{
				UUID:    token.UUID,
				Email:   email,
				TokenID: token.ID,
				Flow:    "xtls-rprx-vision",
			})
		}
	}

	return clients
}

func generateClientUUID(tokenID, nodeID string) string {
	h := md5.New()
	h.Write([]byte(tokenID))
	h.Write([]byte(nodeID))
	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hash[0:8], hash[8:12], hash[12:16], hash[16:20], hash[20:32])
}

func (r *GRPCRuntimeController) addNodeOutbound(ctx context.Context, node domain.Node) error {
	parsed, err := vlesspkg.ParseURL(node.URL)
	if err != nil {
		return fmt.Errorf("parsing vless url: %w", err)
	}

	tag := makeOutboundTag(node.ID)

	var publicKeyBytes []byte
	if parsed.PBK != "" {
		var err error
		publicKeyBytes, err = base64.RawURLEncoding.DecodeString(parsed.PBK)
		if err != nil {
			publicKeyBytes, _ = base64.StdEncoding.DecodeString(parsed.PBK)
		}
	}

	shortIDBytes, _ := hex.DecodeString(parsed.SID)
	if len(shortIDBytes) == 0 && parsed.SID != "" {
		shortIDBytes = []byte(parsed.SID)
	}

	fp := parsed.FP
	if fp == "" {
		fp = "chrome"
	}

	_, err = r.hs.AddOutbound(ctx, &proxymanCommand.AddOutboundRequest{
		Outbound: &core.OutboundHandlerConfig{
			Tag: tag,
			SenderSettings: serial.ToTypedMessage(&proxyman.SenderConfig{
				StreamSettings: &internet.StreamConfig{
					ProtocolName: parsed.Network,
					SecuritySettings: []*serial.TypedMessage{
						serial.ToTypedMessage(&reality.Config{
							Show:        false,
							Fingerprint: fp,
							ServerName:  parsed.SNI,
							PublicKey:   publicKeyBytes,
							ShortId:     shortIDBytes,
							SpiderX:     parsed.SPX,
						}),
					},
					SecurityType: serial.GetMessageType(&reality.Config{}),
				},
			}),
			ProxySettings: serial.ToTypedMessage(&vlessOutbound.Config{
				Vnext: &protocol.ServerEndpoint{
					Address: xnet.NewIPOrDomain(xnet.ParseAddress(parsed.Host)),
					Port:    uint32(parsed.Port),
					User: &protocol.User{
						Account: serial.ToTypedMessage(&vless.Account{
							Id:   parsed.UUID,
							Flow: parsed.Flow,
						}),
					},
				},
			}),
		},
	})

	if err != nil {
		if isAlreadyExistsError(err) {
			r.logger.Debug("outbound already exists", slog.String("tag", tag))
			return nil
		}
		return fmt.Errorf("add outbound: %w", err)
	}

	r.logger.Debug("added outbound", slog.String("tag", tag), slog.String("node_id", node.ID))
	return nil
}

func (r *GRPCRuntimeController) addClientToInbound(ctx context.Context, client ClientInfo) error {
	_, err := r.hs.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
		Tag: r.inboundTag,
		Operation: serial.ToTypedMessage(&proxymanCommand.AddUserOperation{
			User: &protocol.User{
				Level: 0,
				Email: client.Email,
				Account: serial.ToTypedMessage(&vless.Account{
					Id:   client.UUID,
					Flow: client.Flow,
				}),
			},
		}),
	})

	if err != nil {
		if isAlreadyExistsError(err) {
			r.logger.Debug("user already exists", slog.String("email", client.Email))
			return nil
		}
		return fmt.Errorf("add user: %w", err)
	}

	r.logger.Debug("added user to inbound", slog.String("email", client.Email), slog.String("inbound", r.inboundTag))
	return nil
}

func (r *GRPCRuntimeController) addRoutingRule(ctx context.Context, client ClientInfo) error {
	var targetTag string
	if client.NodeID == "" {
		targetTag = "block"
	} else {
		targetTag = makeOutboundTag(client.NodeID)
	}

	ruleTag := "rule-" + client.Email

	cfg := &router.Config{
		Rule: []*router.RoutingRule{
			{
				RuleTag:   ruleTag,
				UserEmail: []string{client.Email},
				TargetTag: &router.RoutingRule_Tag{
					Tag: targetTag,
				},
			},
		},
	}

	_, err := r.rs.AddRule(ctx, &routerCommand.AddRuleRequest{
		Config:       serial.ToTypedMessage(cfg),
		ShouldAppend: true,
	})

	if err != nil {
		if isAlreadyExistsError(err) {
			r.logger.Debug("rule already exists", slog.String("rule_tag", ruleTag))
			return nil
		}
		return fmt.Errorf("add rule: %w", err)
	}

	r.logger.Debug("added routing rule",
		slog.String("rule_tag", ruleTag),
		slog.String("email", client.Email),
		slog.String("outbound", targetTag),
	)
	return nil
}

// ensureInbound creates the VLESS Reality inbound if it doesn't exist.
func (r *GRPCRuntimeController) ensureInbound(ctx context.Context) error {
	if r.inboundSNI == "" {
		return fmt.Errorf("inbound SNI is required for Reality inbound")
	}
	if r.privateKey == "" {
		return fmt.Errorf("private key is required for Reality inbound")
	}

	dest := normalizeRealityDest(r.destination, r.inboundSNI)
	privateKeyBytes := decodeRealityPrivateKey(r.privateKey)

	// Validate dest format (should be host:port)
	if dest == "" || !strings.Contains(dest, ":") {
		r.logger.Error("invalid Reality dest format",
			slog.String("raw_dest", r.destination),
			slog.String("normalized_dest", dest),
			slog.String("sni", r.inboundSNI),
		)
		return fmt.Errorf("invalid dest format: %q (must be host:port)", dest)
	}

	r.logger.Info("Reality inbound parameters",
		slog.String("tag", r.inboundTag),
		slog.String("listen", r.inboundListen),
		slog.Int("port", r.inboundPort),
		slog.String("sni", r.inboundSNI),
		slog.String("dest", dest),
		slog.String("raw_dest", r.destination),
		slog.Int("private_key_len", len(privateKeyBytes)),
	)

	// Remove existing inbound to ensure clean recreation with correct config
	_, removeErr := r.hs.RemoveInbound(ctx, &proxymanCommand.RemoveInboundRequest{
		Tag: r.inboundTag,
	})
	if removeErr == nil {
		r.logger.Debug("removed existing inbound", slog.String("tag", r.inboundTag))
	}

	port := xnet.Port(r.inboundPort)

	_, err := r.hs.AddInbound(ctx, &proxymanCommand.AddInboundRequest{
		Inbound: &core.InboundHandlerConfig{
			Tag: r.inboundTag,
			ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
				Listen: xnet.NewIPOrDomain(xnet.ParseAddress(r.inboundListen)),
				PortList: &xnet.PortList{Range: []*xnet.PortRange{
					{From: uint32(port), To: uint32(port)},
				}},
				StreamSettings: &internet.StreamConfig{
					ProtocolName: "tcp",
					SecuritySettings: []*serial.TypedMessage{
						serial.ToTypedMessage(&reality.Config{
							Show:        false,
							Dest:        dest,
							Type:        "tcp",
							Xver:        0,
							ServerNames: []string{r.inboundSNI},
							PrivateKey:  privateKeyBytes,
							ShortIds:    shortIDBytes(r.shortID),
							Fingerprint: "chrome",
						}),
					},
					SecurityType: serial.GetMessageType(&reality.Config{}),
				},
			}),
			ProxySettings: serial.ToTypedMessage(&vlessInbound.Config{
				Clients:    []*protocol.User{}, // Empty initially, users added separately
				Decryption: "none",
			}),
		},
	})

	if err != nil {
		if isAlreadyExistsError(err) {
			r.logger.Debug("inbound already exists", slog.String("tag", r.inboundTag))
			return nil
		}
		r.logger.Error("failed to add inbound", slog.String("tag", r.inboundTag), slog.String("error", err.Error()))
		return fmt.Errorf("add inbound: %w", err)
	}

	r.logger.Info("created inbound", slog.String("tag", r.inboundTag), slog.Int("port", r.inboundPort))
	return nil
}

// shortIDBytes converts a hex shortID string to [][]byte for Reality inbound config.
func shortIDBytes(sid string) [][]byte {
	if sid == "" {
		return [][]byte{{}}
	}
	b, err := hex.DecodeString(sid)
	if err != nil || len(b) == 0 {
		return [][]byte{[]byte(sid)}
	}
	return [][]byte{b}
}

func normalizeRealityDest(dest, fallbackSNI string) string {
	d := strings.TrimSpace(dest)
	if d == "" {
		d = strings.TrimSpace(fallbackSNI)
	}

	if strings.HasPrefix(d, "http://") || strings.HasPrefix(d, "https://") {
		if u, err := url.Parse(d); err == nil {
			d = u.Host
		}
	}

	if strings.HasPrefix(d, "tcp://") || strings.HasPrefix(d, "udp://") {
		if u, err := url.Parse(d); err == nil {
			d = u.Host
		}
	}

	d = strings.TrimPrefix(d, "tcp:")
	d = strings.TrimPrefix(d, "udp:")
	d = strings.TrimPrefix(d, "//")

	if d != "" && !strings.Contains(d, ":") {
		d += ":443"
	}

	return d
}

func decodeRealityPrivateKey(raw string) []byte {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil
	}

	if b, err := base64.RawURLEncoding.DecodeString(v); err == nil {
		return b
	}
	if b, err := base64.StdEncoding.DecodeString(v); err == nil {
		return b
	}

	return []byte(v)
}

func makeOutboundTag(nodeID string) string {
	return "out-" + sanitizeTag(nodeID)
}

func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsAny(errStr, "already exists", "duplicate", "exists")
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsAny(errStr, "not found", "doesn't exist", "does not exist", "not exist")
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// parseGRPCTarget normalizes admin URL into gRPC target.
// Accepts host:port or host (defaults to port 10085).
func parseGRPCTarget(adminURL string) (string, error) {
	adminURL = strings.TrimSpace(adminURL)
	if adminURL == "" {
		return "", fmt.Errorf("empty admin URL")
	}
	// If no port specified, default to 10085
	if !strings.Contains(adminURL, ":") {
		return adminURL + ":10085", nil
	}
	return adminURL, nil
}
