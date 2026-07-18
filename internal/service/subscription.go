package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"outless/internal/domain"
	"outless/internal/utils"
	"outless/shared/template"
	"outless/shared/vless"

	"gopkg.in/yaml.v3"
)

const (
	defaultFlow        = "xtls-rprx-vision"
	defaultSNI         = "www.google.com"
	defaultFingerprint = "chrome"
	defaultShortID     = "0000000000000000"
)

// HubConfig describes the Hub endpoint clients connect to.
type HubConfig struct {
	Port         int
	SNI          string
	Handshake    string
	APIKey       string
	PublicKey    string
	ShortID      string
	Fingerprint  string
	NameTemplate string
}

// SubscriptionService prepares subscription payloads.
type SubscriptionService struct {
	repo         domain.NodeRepository
	tokenRepo    domain.TokenRepository
	groupRepo    domain.GroupRepository
	inboundRepo  domain.InboundRepository
	externalHost string
	logger       *slog.Logger
	groupCache   map[string]cachedGroupNames
	groupCacheMu sync.RWMutex
}

type cachedGroupNames struct {
	data      map[string]string
	expiresAt time.Time
}

// NewSubscriptionService constructs a subscription service.
func NewSubscriptionService(
	repo domain.NodeRepository,
	tokenRepo domain.TokenRepository,
	groupRepo domain.GroupRepository,
	inboundRepo domain.InboundRepository,
	externalHost string,
	logger *slog.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		repo:         repo,
		tokenRepo:    tokenRepo,
		groupRepo:    groupRepo,
		inboundRepo:  inboundRepo,
		externalHost: externalHost,
		logger:       logger,
		groupCache:   make(map[string]cachedGroupNames),
	}
}

// BuildBase64VLESS returns base64 encoded list of Hub-pointing VLESS URLs.
// If inboundID is empty, uses token.InboundIDs when present (mixing multiple
// inbounds), otherwise falls back to all configured inbounds.
func (s *SubscriptionService) BuildBase64VLESS(ctx context.Context, token string, inboundID string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}
	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	groupSettings, err := s.loadGroupSettings(ctx)
	if err != nil {
		return "", err
	}

	nodes, err := s.repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("loading nodes metadata: %w", err)
	}
	nodes = filterActiveNodes(nodes, now)

	groupNames, err := s.loadGroupNames(ctx)
	if err != nil {
		return "", err
	}

	var hubs []HubConfig
	if inboundID != "" {
		hub, err := s.resolveInbound(ctx, inboundID)
		if err != nil {
			return "", err
		}
		hubs = []HubConfig{hub}
	} else {
		hubs, err = s.resolveInboundsForToken(ctx, tokenInfo)
		if err != nil {
			return "", err
		}
	}

	var allURLs []string
	for _, hub := range hubs {
		hubURLs := s.buildHubURLsWithGroupSettings(tokenInfo, nodes, groupNames, groupSettings, hub)
		allURLs = append(allURLs, hubURLs...)
	}

	if len(allURLs) == 0 {
		s.logger.Warn("no hub URLs generated for token", slog.String("token_id", tokenInfo.ID))
		return "", nil
	}

	payload := strings.Join(allURLs, "\n")
	return base64.StdEncoding.EncodeToString([]byte(payload)), nil
}

func filterActiveNodes(nodes []domain.Node, now time.Time) []domain.Node {
	active := make([]domain.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.ExpiresAt != nil && n.ExpiresAt.Before(now) {
			continue
		}
		active = append(active, n)
	}
	return active
}

func (s *SubscriptionService) resolveInbound(ctx context.Context, inboundID string) (HubConfig, error) {
	inbounds, err := s.inboundRepo.List(ctx)
	if err != nil {
		return HubConfig{}, fmt.Errorf("loading inbounds: %w", err)
	}
	if len(inbounds) == 0 {
		return HubConfig{}, nil
	}

	if inboundID == "" {
		return toHubConfig(inbounds[0]), nil
	}

	for _, inbound := range inbounds {
		if inbound.ID == inboundID {
			return toHubConfig(inbound), nil
		}
	}
	return HubConfig{}, nil
}

func (s *SubscriptionService) resolveInboundsForToken(ctx context.Context, token domain.Token) ([]HubConfig, error) {
	inbounds, err := s.inboundRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading inbounds: %w", err)
	}
	if len(inbounds) == 0 {
		return []HubConfig{}, nil
	}

	// If token has no inbound restrictions, return all inbounds.
	if len(token.InboundIDs) == 0 {
		hubs := make([]HubConfig, 0, len(inbounds))
		for _, inbound := range inbounds {
			hubs = append(hubs, toHubConfig(inbound))
		}
		return hubs, nil
	}

	allowed := make(map[string]struct{}, len(token.InboundIDs))
	for _, id := range token.InboundIDs {
		allowed[id] = struct{}{}
	}

	var hubs []HubConfig
	for _, inbound := range inbounds {
		if _, ok := allowed[inbound.ID]; ok {
			hubs = append(hubs, toHubConfig(inbound))
		}
	}
	if len(hubs) == 0 {
		return nil, fmt.Errorf("token has inbound restrictions but none match configured inbounds")
	}
	return hubs, nil
}

func toHubConfig(inbound domain.Inbound) HubConfig {
	return HubConfig{
		Port:         inbound.Port,
		SNI:          inbound.SNI,
		Handshake:    inbound.Handshake,
		PublicKey:    inbound.PublicKey,
		ShortID:      inbound.ShortID,
		Fingerprint:  inbound.Fingerprint,
		NameTemplate: inbound.NameTemplate,
	}
}

func (s *SubscriptionService) buildNodeRemark(
	node domain.Node,
	groupLabel string,
	hub HubConfig,
	token domain.Token,
) (string, bool) {
	if node.IsSelf {
		if hub.NameTemplate != "" {
			vlessData := template.VLESSData{
				Name:       "Self",
				Host:       s.externalHost,
				Port:       hub.Port,
				SNI:        hub.SNI,
				Security:   "reality",
				Encryption: "none",
				Flow:       defaultFlow,
				FP:         hub.Fingerprint,
			}
			templateData := template.BuildTemplateData(
				vlessData,
				normalizeCountry(node.Country),
				normalizeCountry(node.Country),
				groupLabel,
				token.Owner,
				token.CreatedAt,
				token.ExpiresAt,
			)
			return template.RenderTemplate(hub.NameTemplate, templateData), true
		}
		return buildConnectionRemark(groupLabel, "Self", normalizeCountry(node.Country)), true
	}

	if node.URL == "" {
		return "", false
	}

	parsed, err := vless.ParseURL(node.URL)
	if err != nil {
		s.logger.Warn("failed to parse VLESS URL", slog.String("node_id", node.ID), slog.String("error", err.Error()))
		return "", false
	}

	if hub.NameTemplate != "" {
		vlessData := template.VLESSData{
			Name:       parsed.Name,
			Host:       parsed.Host,
			Port:       parsed.Port,
			SNI:        parsed.SNI,
			Security:   parsed.Security,
			Encryption: parsed.Encryption,
			Flow:       parsed.Flow,
			FP:         parsed.FP,
		}
		templateData := template.BuildTemplateData(
			vlessData,
			normalizeCountry(node.Country),
			normalizeCountry(node.Country),
			groupLabel,
			token.Owner,
			token.CreatedAt,
			token.ExpiresAt,
		)
		return template.RenderTemplate(hub.NameTemplate, templateData), true
	}

	hostLabel := extractNodeHost(node.URL)
	return buildConnectionRemark(groupLabel, hostLabel, normalizeCountry(node.Country)), true
}

func (s *SubscriptionService) buildHubURLs(
	token domain.Token,
	allNodes []domain.Node,
	groupNames map[string]string,
	hub HubConfig,
) []string {
	urls := make([]string, 0, len(allNodes))
	allowedGroups := make(map[string]struct{}, len(token.GroupIDs))
	for _, groupID := range token.GroupIDs {
		allowedGroups[groupID] = struct{}{}
	}
	if len(allowedGroups) == 0 && token.GroupID != "" {
		allowedGroups[token.GroupID] = struct{}{}
	}
	allGroupsAllowed := len(allowedGroups) == 0

	for _, node := range allNodes {
		if !allGroupsAllowed {
			allowed := false
			for _, gid := range node.GroupIDs {
				if _, ok := allowedGroups[gid]; ok {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		primaryGroup := ""
		if len(node.GroupIDs) > 0 {
			primaryGroup = node.GroupIDs[0]
		}
		groupLabel := resolveGroupLabel(groupNames, primaryGroup)

		remark, ok := s.buildNodeRemark(node, groupLabel, hub, token)
		if !ok {
			continue
		}

		uuid := utils.GenerateUUIDFromTokenNode(token.ID, node.ID)
		urls = append(urls, s.formatVLESSURL(uuid, remark, hub))
	}

	return urls
}

func (s *SubscriptionService) buildHubURLsWithGroupSettings(
	token domain.Token,
	allNodes []domain.Node,
	groupNames map[string]string,
	groupSettings map[string]domain.Group,
	hub HubConfig,
) []string {
	allowedGroups := make(map[string]struct{}, len(token.GroupIDs))
	for _, groupID := range token.GroupIDs {
		allowedGroups[groupID] = struct{}{}
	}
	if len(allowedGroups) == 0 && token.GroupID != "" {
		allowedGroups[token.GroupID] = struct{}{}
	}
	allGroupsAllowed := len(allowedGroups) == 0

	nodesByGroup := make(map[string][]domain.Node)
	for _, node := range allNodes {
		if !allGroupsAllowed {
			allowed := false
			for _, gid := range node.GroupIDs {
				if _, ok := allowedGroups[gid]; ok {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		for _, gid := range node.GroupIDs {
			nodesByGroup[gid] = append(nodesByGroup[gid], node)
		}
	}

	var selectedNodes []domain.Node
	for groupID, nodes := range nodesByGroup {
		settings := groupSettings[groupID]
		groupNodes := nodes
		if settings.RandomEnabled {
			shuffleNodes(groupNodes)
		}
		if settings.RandomLimit != nil && *settings.RandomLimit > 0 && len(groupNodes) > *settings.RandomLimit {
			groupNodes = groupNodes[:*settings.RandomLimit]
		}
		selectedNodes = append(selectedNodes, groupNodes...)
	}

	urls := s.buildHubURLs(token, selectedNodes, groupNames, hub)

	return urls
}

func shuffleNodes(nodes []domain.Node) {
	for i := len(nodes) - 1; i > 0; i-- {
		j := int(time.Now().UnixNano()) % (i + 1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

func (s *SubscriptionService) formatVLESSURL(uuid string, remark string, hub HubConfig) string {
	host := s.externalHost
	if host == "" {
		host = "hub.example.com"
	}
	port := hub.Port
	if port == 0 {
		port = 443
	}
	sni := hub.SNI
	if sni == "" {
		sni = hub.Handshake
	}
	if sni == "" {
		sni = defaultSNI
	}
	fingerprint := hub.Fingerprint
	if fingerprint == "" {
		fingerprint = defaultFingerprint
	}

	params := url.Values{}
	params.Set("encryption", "none")
	params.Set("security", "reality")
	params.Set("type", "tcp")
	params.Set("flow", defaultFlow)
	params.Set("sni", sni)
	params.Set("fp", fingerprint)
	if hub.PublicKey != "" {
		params.Set("pbk", hub.PublicKey)
	}
	sid := hub.ShortID
	if sid == "" {
		sid = defaultShortID
	}
	params.Set("sid", sid)

	return fmt.Sprintf("vless://%s@%s:%s?%s#%s",
		uuid, host, strconv.Itoa(port), params.Encode(), url.PathEscape(remark))
}

func normalizeCountry(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "XX"
	}
	return strings.ToUpper(code)
}

// InvalidateGroupCache clears the cached group names.
func (s *SubscriptionService) InvalidateGroupCache() {
	s.groupCacheMu.Lock()
	s.groupCache = make(map[string]cachedGroupNames)
	s.groupCacheMu.Unlock()
}

func (s *SubscriptionService) loadGroupNames(ctx context.Context) (map[string]string, error) {
	const cacheKey = "groups"
	const cacheTTL = 30 * time.Second

	s.groupCacheMu.RLock()
	cached, ok := s.groupCache[cacheKey]
	s.groupCacheMu.RUnlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.data, nil
	}

	groups, err := s.groupRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading groups metadata: %w", err)
	}

	names := make(map[string]string, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(group.ID) == "" {
			continue
		}
		name := strings.TrimSpace(group.Name)
		if name == "" {
			name = group.ID
		}
		names[group.ID] = name
	}

	s.groupCacheMu.Lock()
	s.groupCache[cacheKey] = cachedGroupNames{data: names, expiresAt: time.Now().Add(cacheTTL)}
	s.groupCacheMu.Unlock()

	return names, nil
}

func (s *SubscriptionService) loadGroupSettings(ctx context.Context) (map[string]domain.Group, error) {
	groups, err := s.groupRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading groups metadata: %w", err)
	}
	settings := make(map[string]domain.Group, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(group.ID) == "" {
			continue
		}
		settings[group.ID] = group
	}
	return settings, nil
}

func resolveGroupLabel(groupNames map[string]string, groupID string) string {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return "ungrouped"
	}
	if name, ok := groupNames[groupID]; ok && strings.TrimSpace(name) != "" {
		return name
	}
	return groupID
}

func extractNodeHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "unknown-host"
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "unknown-host"
	}
	return host
}

func buildConnectionRemark(groupName string, host string, country string) string {
	groupName = sanitizeRemarkPart(groupName, "ungrouped")
	host = sanitizeRemarkPart(host, "unknown-host")
	country = sanitizeRemarkPart(country, "XX")
	return fmt.Sprintf("%s | %s | %s", groupName, host, country)
}

func sanitizeRemarkPart(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_")
	value = replacer.Replace(value)
	if ip := net.ParseIP(value); ip != nil {
		return ip.String()
	}
	return value
}

//nolint:unused
func countryFlagEmoji(code string) string {
	if len(code) != 2 {
		return "🏳️"
	}
	code = strings.ToUpper(code)
	first := rune(code[0])
	second := rune(code[1])
	if first < 'A' || first > 'Z' || second < 'A' || second > 'Z' {
		return "🏳️"
	}
	const regionalIndicatorA = rune(0x1F1E6)
	return string([]rune{
		regionalIndicatorA + (first - 'A'),
		regionalIndicatorA + (second - 'A'),
	})
}

// ClashMetaProxy represents a proxy in Clash Meta YAML format.
type ClashMetaProxy struct {
	Name        string            `yaml:"name"`
	Type        string            `yaml:"type"`
	Server      string            `yaml:"server"`
	Port        int               `yaml:"port"`
	UUID        string            `yaml:"uuid"`
	UDP         bool              `yaml:"udp"`
	Flow        string            `yaml:"flow,omitempty"`
	Network     string            `yaml:"network,omitempty"`
	TLS         bool              `yaml:"tls"`
	ServerName  string            `yaml:"servername,omitempty"`
	Fingerprint string            `yaml:"fingerprint,omitempty"`
	RealityOpts *ClashRealityOpts `yaml:"reality-opts,omitempty"`
	Encryption  string            `yaml:"encryption,omitempty"`
}

// ClashRealityOpts contains Reality options for Clash Meta.
type ClashRealityOpts struct {
	PublicKey string `yaml:"public-key"`
	ShortID   string `yaml:"short-id"`
}

// ClashMetaConfig represents the full Clash Meta YAML configuration.
type ClashMetaConfig struct {
	Proxies     []ClashMetaProxy  `yaml:"proxies"`
	ProxyGroups []ClashProxyGroup `yaml:"proxy-groups"`
}

// ClashProxyGroup represents a proxy group in Clash Meta.
type ClashProxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

// SingBoxOutbound represents an outbound in Sing-box JSON format.
type SingBoxOutbound struct {
	Type           string      `json:"type"`
	Tag            string      `json:"tag"`
	Server         string      `json:"server"`
	ServerPort     int         `json:"server_port"`
	UUID           string      `json:"uuid"`
	Flow           string      `json:"flow,omitempty"`
	Network        string      `json:"network,omitempty"`
	TLS            *SingBoxTLS `json:"tls,omitempty"`
	PacketEncoding string      `json:"packet_encoding,omitempty"`
}

// SingBoxTLS contains TLS configuration for Sing-box.
type SingBoxTLS struct {
	Enabled    bool            `json:"enabled"`
	ServerName string          `json:"server_name,omitempty"`
	UTLS       *SingBoxUTLS    `json:"utls,omitempty"`
	Reality    *SingBoxReality `json:"reality,omitempty"`
}

// SingBoxUTLS contains uTLS fingerprint configuration.
type SingBoxUTLS struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// SingBoxReality contains Reality configuration for Sing-box.
type SingBoxReality struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id"`
}

// SingBoxConfig represents the full Sing-box JSON configuration.
type SingBoxConfig struct {
	Outbounds []SingBoxOutbound `json:"outbounds"`
	Route     *SingBoxRoute     `json:"route,omitempty"`
	DNS       *SingBoxDNS       `json:"dns,omitempty"`
}

// SingBoxRoute contains routing configuration.
type SingBoxRoute struct {
	Rules []map[string]any `json:"rules,omitempty"`
}

// SingBoxDNS contains DNS configuration.
type SingBoxDNS struct {
	Servers []map[string]any `json:"servers,omitempty"`
}

// BuildClashMetaYAML generates Clash Meta YAML subscription.
func (s *SubscriptionService) BuildClashMetaYAML(ctx context.Context, token string, inboundID string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}
	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	groupSettings, err := s.loadGroupSettings(ctx)
	if err != nil {
		return "", err
	}

	nodes, err := s.repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("loading nodes metadata: %w", err)
	}

	groupNames, err := s.loadGroupNames(ctx)
	if err != nil {
		return "", err
	}

	var hubs []HubConfig
	if inboundID != "" {
		hub, err := s.resolveInbound(ctx, inboundID)
		if err != nil {
			return "", err
		}
		hubs = []HubConfig{hub}
	} else {
		hubs, err = s.resolveInboundsForToken(ctx, tokenInfo)
		if err != nil {
			return "", err
		}
	}

	config := ClashMetaConfig{
		Proxies:     []ClashMetaProxy{},
		ProxyGroups: []ClashProxyGroup{},
	}

	proxyNames := []string{}

	for _, hub := range hubs {
		selectedNodes := s.getSelectedNodes(tokenInfo, nodes, groupSettings)
		for _, node := range selectedNodes {
			proxy, name := s.buildClashMetaProxy(node, groupNames, hub, tokenInfo)
			config.Proxies = append(config.Proxies, proxy)
			proxyNames = append(proxyNames, name)
		}
	}

	if len(config.Proxies) > 0 {
		config.ProxyGroups = []ClashProxyGroup{
			{
				Name:    "🚀 节点选择",
				Type:    "select",
				Proxies: append([]string{"♻️ 自动选择", "DIRECT"}, proxyNames...),
			},
			{
				Name:    "♻️ 自动选择",
				Type:    "url-test",
				Proxies: proxyNames,
			},
		}
	}

	output, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshaling yaml: %w", err)
	}

	return string(output), nil
}

// BuildSingBoxJSON generates Sing-box JSON subscription.
func (s *SubscriptionService) BuildSingBoxJSON(ctx context.Context, token string, inboundID string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}
	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	groupSettings, err := s.loadGroupSettings(ctx)
	if err != nil {
		return "", err
	}

	nodes, err := s.repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("loading nodes metadata: %w", err)
	}

	groupNames, err := s.loadGroupNames(ctx)
	if err != nil {
		return "", err
	}

	var hubs []HubConfig
	if inboundID != "" {
		hub, err := s.resolveInbound(ctx, inboundID)
		if err != nil {
			return "", err
		}
		hubs = []HubConfig{hub}
	} else {
		hubs, err = s.resolveInboundsForToken(ctx, tokenInfo)
		if err != nil {
			return "", err
		}
	}

	config := SingBoxConfig{
		Outbounds: []SingBoxOutbound{},
		Route: &SingBoxRoute{
			Rules: []map[string]any{
				{
					"outbound": "proxy",
				},
			},
		},
		DNS: &SingBoxDNS{
			Servers: []map[string]any{
				{
					"tag":     "dns-remote",
					"address": "8.8.8.8",
					"detour":  "proxy",
				},
			},
		},
	}

	for _, hub := range hubs {
		selectedNodes := s.getSelectedNodes(tokenInfo, nodes, groupSettings)
		for _, node := range selectedNodes {
			outbound := s.buildSingBoxOutbound(node, groupNames, hub, tokenInfo)
			config.Outbounds = append(config.Outbounds, outbound)
		}
	}

	if len(config.Outbounds) > 0 {
		config.Route.Rules[0]["outbound"] = config.Outbounds[0].Tag
	}

	output, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshaling json: %w", err)
	}

	return string(output), nil
}

// BuildV2RayBase64 generates V2Ray Base64 subscription (list of vless:// URIs).
func (s *SubscriptionService) BuildV2RayBase64(ctx context.Context, token string, inboundID string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}
	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	groupSettings, err := s.loadGroupSettings(ctx)
	if err != nil {
		return "", err
	}

	nodes, err := s.repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("loading nodes metadata: %w", err)
	}

	groupNames, err := s.loadGroupNames(ctx)
	if err != nil {
		return "", err
	}

	var hubs []HubConfig
	if inboundID != "" {
		hub, err := s.resolveInbound(ctx, inboundID)
		if err != nil {
			return "", err
		}
		hubs = []HubConfig{hub}
	} else {
		hubs, err = s.resolveInboundsForToken(ctx, tokenInfo)
		if err != nil {
			return "", err
		}
	}

	var allURLs []string
	for _, hub := range hubs {
		selectedNodes := s.getSelectedNodes(tokenInfo, nodes, groupSettings)
		for _, node := range selectedNodes {
			url := s.buildV2RayURL(node, groupNames, hub, tokenInfo)
			if url != "" {
				allURLs = append(allURLs, url)
			}
		}
	}

	if len(allURLs) == 0 {
		return "", nil
	}

	payload := strings.Join(allURLs, "\n")
	return base64.StdEncoding.EncodeToString([]byte(payload)), nil
}

// BuildSurgeConf generates Surge configuration.
func (s *SubscriptionService) BuildSurgeConf(ctx context.Context, token string, inboundID string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}
	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	groupSettings, err := s.loadGroupSettings(ctx)
	if err != nil {
		return "", err
	}

	nodes, err := s.repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("loading nodes metadata: %w", err)
	}

	groupNames, err := s.loadGroupNames(ctx)
	if err != nil {
		return "", err
	}

	var hubs []HubConfig
	if inboundID != "" {
		hub, err := s.resolveInbound(ctx, inboundID)
		if err != nil {
			return "", err
		}
		hubs = []HubConfig{hub}
	} else {
		hubs, err = s.resolveInboundsForToken(ctx, tokenInfo)
		if err != nil {
			return "", err
		}
	}

	var lines []string
	lines = append(lines, "[Proxy]")

	proxyNames := []string{}
	for _, hub := range hubs {
		selectedNodes := s.getSelectedNodes(tokenInfo, nodes, groupSettings)
		for _, node := range selectedNodes {
			line, name := s.buildSurgeProxy(node, groupNames, hub, tokenInfo)
			if line != "" {
				lines = append(lines, line)
				proxyNames = append(proxyNames, name)
			}
		}
	}

	if len(proxyNames) > 0 {
		lines = append(lines, "")
		lines = append(lines, "[Proxy Group]")
		lines = append(lines, fmt.Sprintf("🚀 节点选择 = select, %s, DIRECT", strings.Join(proxyNames, ", ")))
	}

	return strings.Join(lines, "\n"), nil
}

func (s *SubscriptionService) getSelectedNodes(
	token domain.Token,
	allNodes []domain.Node,
	groupSettings map[string]domain.Group,
) []domain.Node {
	allowedGroups := make(map[string]struct{}, len(token.GroupIDs))
	for _, groupID := range token.GroupIDs {
		allowedGroups[groupID] = struct{}{}
	}
	if len(allowedGroups) == 0 && token.GroupID != "" {
		allowedGroups[token.GroupID] = struct{}{}
	}
	allGroupsAllowed := len(allowedGroups) == 0

	nodesByGroup := make(map[string][]domain.Node)
	for _, node := range allNodes {
		if !allGroupsAllowed {
			allowed := false
			for _, gid := range node.GroupIDs {
				if _, ok := allowedGroups[gid]; ok {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		for _, gid := range node.GroupIDs {
			nodesByGroup[gid] = append(nodesByGroup[gid], node)
		}
	}

	var selectedNodes []domain.Node
	for groupID, nodes := range nodesByGroup {
		settings := groupSettings[groupID]
		groupNodes := nodes
		if settings.RandomEnabled {
			shuffleNodes(groupNodes)
		}
		if settings.RandomLimit != nil && *settings.RandomLimit > 0 && len(groupNodes) > *settings.RandomLimit {
			groupNodes = groupNodes[:*settings.RandomLimit]
		}
		selectedNodes = append(selectedNodes, groupNodes...)
	}

	return selectedNodes
}

func (s *SubscriptionService) buildClashMetaProxy(
	node domain.Node,
	groupNames map[string]string,
	hub HubConfig,
	token domain.Token,
) (ClashMetaProxy, string) {
	remark, _ := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if remark == "" {
		remark = node.ID
	}

	uuid := utils.GenerateUUIDFromTokenNode(token.ID, node.ID)

	sni := hub.SNI
	if sni == "" {
		sni = hub.Handshake
	}
	if sni == "" {
		sni = defaultSNI
	}

	fingerprint := hub.Fingerprint
	if fingerprint == "" {
		fingerprint = defaultFingerprint
	}

	shortID := hub.ShortID
	if shortID == "" {
		shortID = defaultShortID
	}

	return ClashMetaProxy{
		Name:        remark,
		Type:        "vless",
		Server:      s.externalHost,
		Port:        hub.Port,
		UUID:        uuid,
		UDP:         true,
		Flow:        defaultFlow,
		Network:     "tcp",
		TLS:         true,
		ServerName:  sni,
		Fingerprint: fingerprint,
		RealityOpts: &ClashRealityOpts{
			PublicKey: hub.PublicKey,
			ShortID:   shortID,
		},
		Encryption: "",
	}, remark
}

func (s *SubscriptionService) buildSingBoxOutbound(
	node domain.Node,
	groupNames map[string]string,
	hub HubConfig,
	token domain.Token,
) SingBoxOutbound {
	remark, _ := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if remark == "" {
		remark = node.ID
	}

	uuid := utils.GenerateUUIDFromTokenNode(token.ID, node.ID)

	sni := hub.SNI
	if sni == "" {
		sni = hub.Handshake
	}
	if sni == "" {
		sni = defaultSNI
	}

	fingerprint := hub.Fingerprint
	if fingerprint == "" {
		fingerprint = defaultFingerprint
	}

	shortID := hub.ShortID
	if shortID == "" {
		shortID = defaultShortID
	}

	return SingBoxOutbound{
		Type:           "vless",
		Tag:            remark,
		Server:         s.externalHost,
		ServerPort:     hub.Port,
		UUID:           uuid,
		Flow:           defaultFlow,
		Network:        "tcp",
		PacketEncoding: "xudp",
		TLS: &SingBoxTLS{
			Enabled:    true,
			ServerName: sni,
			UTLS: &SingBoxUTLS{
				Enabled:     true,
				Fingerprint: fingerprint,
			},
			Reality: &SingBoxReality{
				Enabled:   true,
				PublicKey: hub.PublicKey,
				ShortID:   shortID,
			},
		},
	}
}

func (s *SubscriptionService) buildV2RayURL(node domain.Node, groupNames map[string]string, hub HubConfig, token domain.Token) string {
	remark, ok := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if !ok {
		return ""
	}

	uuid := utils.GenerateUUIDFromTokenNode(token.ID, node.ID)
	return s.formatVLESSURL(uuid, remark, hub)
}

func (s *SubscriptionService) buildSurgeProxy(
	node domain.Node,
	groupNames map[string]string,
	hub HubConfig,
	token domain.Token,
) (string, string) {
	remark, ok := s.buildNodeRemark(node, resolveGroupLabel(groupNames, getNodePrimaryGroup(node)), hub, token)
	if !ok {
		return "", ""
	}

	uuid := utils.GenerateUUIDFromTokenNode(token.ID, node.ID)

	sni := hub.SNI
	if sni == "" {
		sni = hub.Handshake
	}
	if sni == "" {
		sni = defaultSNI
	}

	fingerprint := hub.Fingerprint
	if fingerprint == "" {
		fingerprint = defaultFingerprint
	}

	shortID := hub.ShortID
	if shortID == "" {
		shortID = defaultShortID
	}

	line := fmt.Sprintf("%s = vless, %s, %d, uuid=%s, tls=true, sni=%s, fp=%s, pbk=%s, sid=%s, flow=%s",
		remark, s.externalHost, hub.Port, uuid, sni, fingerprint, hub.PublicKey, shortID, defaultFlow)

	return line, remark
}

func getNodePrimaryGroup(node domain.Node) string {
	if len(node.GroupIDs) > 0 {
		return node.GroupIDs[0]
	}
	return ""
}
