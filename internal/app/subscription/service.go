package subscription

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
)

// HubConfig describes the Hub endpoint clients connect to.
// These values are embedded into every VLESS URL returned by the subscription.
type HubConfig struct {
	Host string
	Port int
	SNI  string
	// Added a new field to the HubConfig struct
	APIKey      string
	PublicKey   string
	ShortID     string
	Fingerprint string
}

// Service prepares subscription payloads.
type Service struct {
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

// NewService constructs a subscription service.
func NewService(repo domain.NodeRepository, tokenRepo domain.TokenRepository, groupRepo domain.GroupRepository, hub HubConfig, logger *slog.Logger) *Service {
	return &Service{
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
func (s *Service) BuildBase64VLESS(ctx context.Context, token string) (string, error) {
	now := time.Now().UTC()

	tokenInfo, err := s.tokenRepo.GetTokenByPlain(ctx, token, now)
	if err != nil {
		return "", err
	}

	if tokenInfo.UUID == "" {
		return "", fmt.Errorf("token %s has no uuid assigned", tokenInfo.ID)
	}

	countries, err := s.repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("loading nodes metadata: %w", err)
	}

	groupNames, err := s.loadGroupNames(ctx)
	if err != nil {
		return "", err
	}

	s.logger.Info(fmt.Sprintf("Building VLESS subscription: token=%s uuid=%s group=%s groups=%v nodes=%d", tokenInfo.ID, tokenInfo.UUID, tokenInfo.GroupID, tokenInfo.GroupIDs, len(countries)))

	hubURLs := s.buildHubURLs(tokenInfo, countries, groupNames)
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
func (s *Service) buildHubURLs(token domain.Token, allNodes []domain.Node, groupNames map[string]string) []string {
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
		if node.Status != domain.NodeStatusHealthy {
			continue
		}
		if !allGroupsAllowed {
			if _, ok := allowedGroups[node.GroupID]; !ok {
				continue
			}
		}
		if node.URL == "" {
			continue
		}

		groupLabel := resolveGroupLabel(groupNames, node.GroupID)
		hostLabel := extractNodeHost(node.URL)
		remark := buildConnectionRemark(groupLabel, hostLabel, normalizeCountry(node.Country), node.Latency)
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

// formatVLESSURL assembles a Reality-ready VLESS URL pointing to the Hub.
func (s *Service) formatVLESSURL(uuid string, remark string) string {
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

func (s *Service) loadGroupNames(ctx context.Context) (map[string]string, error) {
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
	s.groupCache[cacheKey] = cachedGroupNames{
		data:      names,
		expiresAt: time.Now().Add(cacheTTL),
	}
	s.groupCacheMu.Unlock()

	return names, nil
}

// invalidateGroupCache clears the group names cache.
func (s *Service) invalidateGroupCache() {
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
