package subscription

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"outless/internal/domain"
)

// Service prepares subscription payloads.
type Service struct {
	repo      domain.NodeRepository
	tokenRepo domain.TokenRepository
}

// NewService constructs a subscription service.
func NewService(repo domain.NodeRepository, tokenRepo domain.TokenRepository) *Service {
	return &Service{repo: repo, tokenRepo: tokenRepo}
}

// BuildBase64VLESS returns base64 encoded VLESS list filtered by token's group.
func (s *Service) BuildBase64VLESS(ctx context.Context, token string) (string, error) {
	valid, err := s.tokenRepo.ValidateToken(ctx, token, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("validating token: %w", err)
	}
	if !valid {
		return "", domain.ErrUnauthorized
	}

	groupID, err := s.tokenRepo.GetTokenGroupID(ctx, token, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("getting token group id: %w", err)
	}

	urls, err := s.repo.ListVLESSURLs(ctx, groupID)
	if err != nil {
		return "", fmt.Errorf("loading vless urls: %w", err)
	}

	payload := strings.TrimSpace(strings.Join(urls, "\n"))
	if payload == "" {
		return "", nil
	}

	return base64.StdEncoding.EncodeToString([]byte(payload)), nil
}
