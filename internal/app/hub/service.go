package hub

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"outless/internal/domain"
)

// Service handles L4 proxy connections and traffic relay.
type Service struct {
	nodeRepo    domain.NodeRepository
	tokenRepo   domain.TokenRepository
	logger      *slog.Logger
}

// NewService constructs a hub service.
func NewService(nodeRepo domain.NodeRepository, tokenRepo domain.TokenRepository, logger *slog.Logger) *Service {
	return &Service{
		nodeRepo:  nodeRepo,
		tokenRepo: tokenRepo,
		logger:    logger,
	}
}

// HandleConnection processes a client connection.
func (s *Service) HandleConnection(ctx context.Context, conn net.Conn, token string) error {
	defer conn.Close()

	// Validate token
	valid, err := s.tokenRepo.ValidateToken(ctx, token, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("validating token: %w", err)
	}
	if !valid {
		s.logger.Warn("invalid token from hub connection", slog.String("remote_addr", conn.RemoteAddr().String()))
		return fmt.Errorf("invalid token")
	}

	// Get group ID from token
	groupID, err := s.tokenRepo.GetTokenGroupID(ctx, token, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("getting token group id: %w", err)
	}

	// Get healthy nodes from group
	urls, err := s.nodeRepo.ListVLESSURLs(ctx, groupID)
	if err != nil {
		return fmt.Errorf("getting nodes: %w", err)
	}

	if len(urls) == 0 {
		s.logger.Warn("no healthy nodes available", slog.String("group_id", groupID))
		return fmt.Errorf("no healthy nodes available")
	}

	// Select first healthy node (TODO: implement load balancing)
	selectedURL := urls[0]
	s.logger.Info("hub connection established", slog.String("remote_addr", conn.RemoteAddr().String()), slog.String("selected_node", selectedURL))

	// TODO: Connect to Xray and relay traffic
	// For now, just close the connection
	return nil
}

// copyData relays data between two connections bidirectionally.
func (s *Service) copyData(dst net.Conn, src net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	defer dst.Close()
	defer src.Close()

	_, err := io.Copy(dst, src)
	if err != nil {
		s.logger.Debug("copy error", slog.String("error", err.Error()))
	}
}
