package service

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
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
)

// HubConfig describes the Hub endpoint clients connect to.
type HubConfig struct {
	Host               string
	Port               int
	SNI                string
	APIKey             string
	PublicKey          string
	ShortID            string
	Fingerprint        string
	NameTemplate       string
	EnableAutoSelfNode bool
	AutoSelfNodeName   string
}

// SubscriptionService prepares subscription payloads.
type SubscriptionService struct {
	repo         domain.NodeRepository
	tokenRepo    domain.TokenRepository
	groupRepo    domain.GroupRepository
	inboundRepo  domain.InboundRepository
	logger       *slog.Logger
	groupCache   map[string]cachedGroupNames
	groupCacheMu sync.RWMutex
}

type cachedGroupNames struct {
	data      map[string]string
	expiresAt time.Time
}

// NewSubscriptionService constructs a subscription service.
func NewSubscriptionService(repo domain.NodeRepository, tokenRepo domain.TokenRepository, groupRepo domain.GroupRepository, inboundRepo domain.InboundRepository, logger *slog.Logger) *SubscriptionService {
	return &SubscriptionService{
		repo:        repo,
		tokenRepo:   tokenRepo,
		groupRepo:   groupRepo,
		inboundRepo: inboundRepo,
		logger:      logger,
		groupCache:  make(map[string]cachedGroupNames),
	}
}

// BuildBase64VLESS returns base64 encoded list of Hub-pointing VLESS URLs.
// If inboundID is empty, the first stored inbound is used.
func (s *SubscriptionService) BuildBase64VLESS(ctx context.Context, token string, inboundID string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}
	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	hub, err := s.resolveInbound(ctx, inboundID)
	if err != nil {
		return "", err
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

	hubURLs := s.buildHubURLsWithGroupSettings(tokenInfo, nodes, groupNames, groupSettings, hub)
	if len(hubURLs) == 0 {
		s.logger.Warn("no hub URLs generated for token", slog.String("token_id", tokenInfo.ID))
		return "", nil
	}

	payload := strings.Join(hubURLs, "\n")
	return base64.StdEncoding.EncodeToString([]byte(payload)), nil
}

func (s *SubscriptionService) resolveInbound(ctx context.Context, inboundID string) (HubConfig, error) {
	inbounds, err := s.inboundRepo.List(ctx)
	if err != nil {
		return HubConfig{}, fmt.Errorf("loading inbounds: %w", err)
	}
	if len(inbounds) == 0 {
		return HubConfig{}, fmt.Errorf("no inbounds configured")
	}

	if inboundID == "" {
		return toHubConfig(inbounds[0]), nil
	}

	for _, inbound := range inbounds {
		if inbound.ID == inboundID {
			return toHubConfig(inbound), nil
		}
	}
	return HubConfig{}, fmt.Errorf("inbound not found: %s", inboundID)
}

func toHubConfig(inbound domain.Inbound) HubConfig {
	return HubConfig{
		Host:               inbound.URLHost,
		Port:               inbound.Port,
		SNI:                inbound.SNI,
		PublicKey:          inbound.PublicKey,
		ShortID:            inbound.ShortID,
		Fingerprint:        inbound.Fingerprint,
		NameTemplate:       inbound.NameTemplate,
		EnableAutoSelfNode: inbound.EnableAutoSelfNode,
		AutoSelfNodeName:   inbound.AutoSelfNodeName,
	}
}

func generateSelfNodeUUID(tokenID string) string {
	if tokenID == "" {
		return ""
	}
	h := md5.New()
	h.Write([]byte(tokenID))
	h.Write([]byte("__self__"))
	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s-%s-%s-%s-%s", hash[0:8], hash[8:12], hash[12:16], hash[16:20], hash[20:32])
}

func (s *SubscriptionService) buildHubURLs(token domain.Token, allNodes []domain.Node, groupNames map[string]string, hub HubConfig) []string {
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
			if _, ok := allowedGroups[node.GroupID]; !ok {
				continue
			}
		}
		if node.URL == "" {
			continue
		}

		parsed, err := vless.ParseURL(node.URL)
		if err != nil {
			s.logger.Warn("failed to parse VLESS URL", slog.String("node_id", node.ID), slog.String("error", err.Error()))
			continue
		}

		var remark string
		if hub.NameTemplate != "" {
			groupLabel := resolveGroupLabel(groupNames, node.GroupID)
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
			templateData := template.BuildTemplateData(vlessData, groupLabel, normalizeCountry(node.Country), groupLabel, token.Owner)
			remark = template.RenderTemplate(hub.NameTemplate, templateData)
		} else {
			groupLabel := resolveGroupLabel(groupNames, node.GroupID)
			hostLabel := extractNodeHost(node.URL)
			remark = buildConnectionRemark(groupLabel, hostLabel, normalizeCountry(node.Country), 0)
		}

		uuid := utils.GenerateUUIDFromTokenNode(token.ID, node.ID)
		urls = append(urls, s.formatVLESSURL(uuid, remark, hub))
	}

	if len(urls) == 0 {
		s.logger.Warn("no accessible nodes for token, using fallback", slog.String("token_id", token.ID))
		urls = append(urls, s.formatVLESSURL(token.UUID, "Outless", hub))
	}

	return urls
}

func (s *SubscriptionService) buildHubURLsWithGroupSettings(token domain.Token, allNodes []domain.Node, groupNames map[string]string, groupSettings map[string]domain.Group, hub HubConfig) []string {
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
			if _, ok := allowedGroups[node.GroupID]; !ok {
				continue
			}
		}
		if node.URL == "" {
			continue
		}
		nodesByGroup[node.GroupID] = append(nodesByGroup[node.GroupID], node)
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

	if hub.EnableAutoSelfNode {
		selfNodeUUID := generateSelfNodeUUID(token.ID)
		selfNodeName := hub.AutoSelfNodeName
		if selfNodeName == "" {
			selfNodeName = "Direct Exit"
		}
		urls = append(urls, s.formatVLESSURL(selfNodeUUID, selfNodeName, hub))
	}

	return urls
}

func shuffleNodes(nodes []domain.Node) {
	for i := len(nodes) - 1; i > 0; i-- {
		j := int(time.Now().UnixNano()) % (i + 1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

func (s *SubscriptionService) formatVLESSURL(uuid string, remark string, hub HubConfig) string {
	host := hub.Host
	if host == "" {
		host = "hub.example.com"
	}
	port := hub.Port
	if port == 0 {
		port = 443
	}
	sni := hub.SNI
	fingerprint := hub.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
	}

	params := url.Values{}
	params.Set("encryption", "none")
	params.Set("security", "reality")
	params.Set("type", "tcp")
	params.Set("flow", "xtls-rprx-vision")
	params.Set("sni", sni)
	params.Set("fp", fingerprint)
	if hub.PublicKey != "" {
		params.Set("pbk", hub.PublicKey)
	}
	params.Set("sid", hub.ShortID)

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

func buildConnectionRemark(groupName string, host string, country string, latency time.Duration) string {
	groupName = sanitizeRemarkPart(groupName, "ungrouped")
	host = sanitizeRemarkPart(host, "unknown-host")
	country = sanitizeRemarkPart(country, "XX")
	flag := countryFlagEmoji(country)
	latencyMS := latency.Milliseconds()
	if latencyMS < 0 {
		latencyMS = 0
	}
	return fmt.Sprintf("🛰️ %s | 🖥️ %s | 🌍 %s %s | ⚡ %dms", groupName, host, country, flag, latencyMS)
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
