package subscription

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"outless/internal/domain"
)

// HubConfig describes the Hub endpoint clients connect to.
// These values are embedded into every VLESS URL returned by the subscription.
type HubConfig struct {
	Host        string
	Port        int
	SNI         string
	PublicKey   string
	ShortID     string
	Fingerprint string
}

// Service prepares subscription payloads.
type Service struct {
	repo      domain.NodeRepository
	tokenRepo domain.TokenRepository
	hub       HubConfig
	logger    *slog.Logger
}

// NewService constructs a subscription service.
func NewService(repo domain.NodeRepository, tokenRepo domain.TokenRepository, hub HubConfig, logger *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		tokenRepo: tokenRepo,
		hub:       hub,
		logger:    logger,
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

	hubURLs := s.buildHubURLs(tokenInfo, countries)
	if len(hubURLs) == 0 {
		return "", nil
	}

	payload := strings.Join(hubURLs, "\n")
	return base64.StdEncoding.EncodeToString([]byte(payload)), nil
}

// buildHubURLs constructs one VLESS URL per healthy node in the token's groups.
// Each URL points to the Hub (same UUID), with a country-tagged remark so v2rayN
// shows users a meaningful location menu.
func (s *Service) buildHubURLs(token domain.Token, allNodes []domain.Node) []string {
	urls := make([]string, 0, len(allNodes))
	index := 0
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

		index++
		remark := fmt.Sprintf("Outless-%s-%d", normalizeCountry(node.Country), index)
		urls = append(urls, s.formatVLESSURL(token.UUID, remark))
	}

	if len(urls) == 0 {
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
	if sni == "" {
		sni = "www.google.com"
	}
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
	if s.hub.ShortID != "" {
		params.Set("sid", s.hub.ShortID)
	}

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
