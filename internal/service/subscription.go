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
	"outless/shared/template"
	"outless/shared/vless"
)

// HubConfig describes the Hub endpoint clients connect to.
// These values are embedded into every VLESS URL returned by the subscription.
type HubConfig struct {
	Host         string
	Port         int
	SNI          string
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
	hub          HubConfig
	logger       *slog.Logger
	groupCache   map[string]cachedGroupNames
	groupCacheMu sync.RWMutex
}

type cachedGroupNames struct {
	data      map[string]string
	expiresAt time.Time
}

// NewSubscriptionService constructs a subscription service.
func NewSubscriptionService(repo domain.NodeRepository, tokenRepo domain.TokenRepository, groupRepo domain.GroupRepository, hub HubConfig, logger *slog.Logger) *SubscriptionService {
	return &SubscriptionService{
		repo:       repo,
		tokenRepo:  tokenRepo,
		groupRepo:  groupRepo,
		hub:        hub,
		logger:     logger,
		groupCache: make(map[string]cachedGroupNames),
	}
}

// BuildBase64VLESS returns base64 encoded list of Hub-pointing VLESS URLs
// filtered by the token groups. Each entry represents one healthy exit node
// visible to the user. Actual routing to the exit happens server-side inside Hub.
func (s *SubscriptionService) BuildBase64VLESS(ctx context.Context, token string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}

	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	// Load group settings to apply random/limit per group
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

	s.logger.Info(fmt.Sprintf("Building VLESS subscription: token=%s uuid=%s group=%s groups=%v nodes=%d", tokenInfo.ID, tokenInfo.UUID, tokenInfo.GroupID, tokenInfo.GroupIDs, len(nodes)))

	hubURLs := s.buildHubURLsWithGroupSettings(tokenInfo, nodes, groupNames, groupSettings)
	if len(hubURLs) == 0 {
		s.logger.Warn(fmt.Sprintf("No hub URLs generated for token: %s", tokenInfo.ID))
		return "", nil
	}

	s.logger.Info(fmt.Sprintf("Generated VLESS subscription: token=%s urls=%d", tokenInfo.ID, len(hubURLs)))

	payload := strings.Join(hubURLs, "\n")
	return base64.StdEncoding.EncodeToString([]byte(payload)), nil
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

// buildHubURLs constructs one VLESS URL per healthy node in the token's groups.
// Each URL points to the Hub with a unique UUID for that specific node.
func (s *SubscriptionService) buildHubURLs(token domain.Token, allNodes []domain.Node, groupNames map[string]string) []string {
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

		// Parse VLESS URL to extract original data
		parsed, err := vless.ParseURL(node.URL)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("Failed to parse VLESS URL: node=%s error=%s", node.ID, err.Error()))
			continue
		}

		// Generate remark using template or fallback
		var remark string
		if s.hub.NameTemplate != "" {
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
			remark = template.RenderTemplate(s.hub.NameTemplate, templateData)
		} else {
			groupLabel := resolveGroupLabel(groupNames, node.GroupID)
			hostLabel := extractNodeHost(node.URL)
			remark = buildConnectionRemark(groupLabel, hostLabel, normalizeCountry(node.Country), 0)
		}

		// Generate unique UUID for this token+node combination
		uuid := generateUUIDFromTokenNode(token.ID, node.ID)

		s.logger.Info(fmt.Sprintf("Generated VLESS URL: token=%s node=%s group=%s uuid=%s", token.ID, node.ID, node.GroupID, uuid))

		urls = append(urls, s.formatVLESSURL(uuid, remark))
	}

	if len(urls) == 0 {
		s.logger.Warn(fmt.Sprintf("No accessible nodes for token, using fallback: token=%s uuid=%s", token.ID, token.UUID))
		urls = append(urls, s.formatVLESSURL(token.UUID, "Outless"))
	}

	return urls
}

// buildHubURLsWithGroupSettings constructs VLESS URLs with group-specific random/limit settings.
func (s *SubscriptionService) buildHubURLsWithGroupSettings(token domain.Token, allNodes []domain.Node, groupNames map[string]string, groupSettings map[string]domain.Group) []string {
	allowedGroups := make(map[string]struct{}, len(token.GroupIDs))
	for _, groupID := range token.GroupIDs {
		allowedGroups[groupID] = struct{}{}
	}
	if len(allowedGroups) == 0 && token.GroupID != "" {
		allowedGroups[token.GroupID] = struct{}{}
	}
	allGroupsAllowed := len(allowedGroups) == 0

	// Group nodes by group ID to apply random/limit per group
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

		// Apply random selection if enabled
		if settings.RandomEnabled {
			shuffleNodes(groupNodes)
		}

		// Apply limit if set
		if settings.RandomLimit != nil && *settings.RandomLimit > 0 && len(groupNodes) > *settings.RandomLimit {
			groupNodes = groupNodes[:*settings.RandomLimit]
		}

		selectedNodes = append(selectedNodes, groupNodes...)
	}

	// Build URLs from selected nodes
	return s.buildHubURLs(token, selectedNodes, groupNames)
}

// shuffleNodes randomly shuffles a slice of nodes in place using Fisher-Yates algorithm.
func shuffleNodes(nodes []domain.Node) {
	for i := len(nodes) - 1; i > 0; i-- {
		j := int(time.Now().UnixNano()) % (i + 1)
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

// formatVLESSURL assembles a Reality-ready VLESS URL pointing to the Hub.
func (s *SubscriptionService) formatVLESSURL(uuid string, remark string) string {
	host := s.hub.Host
	if host == "" {
		host = "hub.example.com"
	}
	port := s.hub.Port
	if port == 0 {
		port = 443
	}
	sni := s.hub.SNI
	fingerprint := s.hub.Fingerprint
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
	if s.hub.PublicKey != "" {
		params.Set("pbk", s.hub.PublicKey)
	}
	// Always include sid (even if empty) to ensure REALITY handshake compatibility
	params.Set("sid", s.hub.ShortID)

	return fmt.Sprintf("vless://%s@%s:%s?%s#%s",
		uuid,
		host,
		strconv.Itoa(port),
		params.Encode(),
		url.PathEscape(remark),
	)
}

func normalizeCountry(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "XX"
	}
	return strings.ToUpper(code)
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

// invalidateGroupCache clears the group names cache.
func (s *SubscriptionService) invalidateGroupCache() {
	s.groupCacheMu.Lock()
	delete(s.groupCache, "groups")
	s.groupCacheMu.Unlock()
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
