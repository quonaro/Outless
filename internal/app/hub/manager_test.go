package hub

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"iter"

	"outless/internal/adapters/xray"
	"outless/internal/domain"
)

type runtimeControllerStub struct {
	startCalls  int
	reloadCalls int
	stopCalls   int
}

func (s *runtimeControllerStub) Start(string) error {
	s.startCalls++
	return nil
}

func (s *runtimeControllerStub) Reload(string) error {
	s.reloadCalls++
	return nil
}

func (s *runtimeControllerStub) Stop() { s.stopCalls++ }
func (s *runtimeControllerStub) Description() string {
	return "stub"
}

type tokenRepoStub struct {
	tokens []domain.Token
}

func (s *tokenRepoStub) IssueToken(context.Context, string, []string, time.Time) (domain.Token, error) {
	return domain.Token{}, nil
}
func (s *tokenRepoStub) ValidateToken(context.Context, string, time.Time) (bool, error) {
	return false, nil
}
func (s *tokenRepoStub) GetTokenGroupID(context.Context, string, time.Time) (string, error) {
	return "", nil
}
func (s *tokenRepoStub) GetTokenByPlain(context.Context, string, time.Time) (domain.Token, error) {
	return domain.Token{}, nil
}
func (s *tokenRepoStub) ListActive(context.Context, time.Time) ([]domain.Token, error) {
	return append([]domain.Token(nil), s.tokens...), nil
}
func (s *tokenRepoStub) List(context.Context) ([]domain.Token, error) { return nil, nil }
func (s *tokenRepoStub) Deactivate(context.Context, string) error     { return nil }
func (s *tokenRepoStub) Activate(context.Context, string) error       { return nil }
func (s *tokenRepoStub) Remove(context.Context, string) error         { return nil }

type nodeRepoStub struct {
	nodes []domain.Node
}

func (s *nodeRepoStub) IterateNodes(context.Context) iter.Seq2[domain.Node, error] {
	return func(func(domain.Node, error) bool) {}
}
func (s *nodeRepoStub) ListVLESSURLs(context.Context, string) ([]string, error) { return nil, nil }
func (s *nodeRepoStub) UpdateProbeResult(context.Context, domain.ProbeResult) error {
	return nil
}
func (s *nodeRepoStub) Create(context.Context, domain.Node) error { return nil }
func (s *nodeRepoStub) CreateIfAbsent(context.Context, domain.Node) (bool, error) {
	return false, nil
}
func (s *nodeRepoStub) BulkCreateIfAbsent(context.Context, []domain.Node) ([]string, error) {
	return nil, nil
}
func (s *nodeRepoStub) Upsert(context.Context, domain.Node) error { return nil }
func (s *nodeRepoStub) FindByID(context.Context, string) (domain.Node, error) {
	return domain.Node{}, nil
}
func (s *nodeRepoStub) List(context.Context) ([]domain.Node, error) {
	return append([]domain.Node(nil), s.nodes...), nil
}
func (s *nodeRepoStub) ListPage(context.Context, int, int) ([]domain.Node, error) { return nil, nil }
func (s *nodeRepoStub) ListPageByGroup(context.Context, string, int, int) ([]domain.Node, error) {
	return nil, nil
}
func (s *nodeRepoStub) ListByGroup(context.Context, string) ([]domain.Node, error) { return nil, nil }
func (s *nodeRepoStub) ListNonHealthyByGroup(context.Context, string) ([]domain.Node, error) {
	return nil, nil
}
func (s *nodeRepoStub) DeleteUnavailableByGroup(context.Context, string) (int64, error) {
	return 0, nil
}
func (s *nodeRepoStub) Update(context.Context, domain.Node) error { return nil }
func (s *nodeRepoStub) Delete(context.Context, string) error      { return nil }

func TestManagerSync_ReloadsViaRuntimeController(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	tokens := &tokenRepoStub{
		tokens: []domain.Token{
			{ID: "t1", UUID: "11111111-1111-1111-1111-111111111111", IsActive: true, ExpiresAt: now.Add(time.Hour)},
		},
	}
	nodes := &nodeRepoStub{
		nodes: []domain.Node{
			{
				ID:      "n1",
				GroupID: "g1",
				Status:  domain.NodeStatusHealthy,
				URL:     "vless://11111111-1111-1111-1111-111111111111@node.example.com:443?type=tcp&security=tls&sni=www.google.com",
			},
		},
	}
	runtimeStub := &runtimeControllerStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	configPath := filepath.Join(t.TempDir(), "xray-hub.json")

	mgr := NewManager(tokens, nodes, runtimeStub, ManagerConfig{
		ConfigPath: configPath,
		Inbound: xray.HubInboundConfig{
			Listen:      "0.0.0.0",
			Port:        443,
			SNI:         "www.google.com",
			PrivateKey:  "private",
			ShortID:     "abcd1234",
			Destination: "www.google.com:443",
		},
	}, logger)

	if err := mgr.Sync(context.Background()); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if runtimeStub.reloadCalls != 0 {
		t.Fatalf("unexpected reload calls after initial sync: %d", runtimeStub.reloadCalls)
	}

	// Simulate runtime started after run() to verify reload path.
	mgr.runtimeActive = true
	tokens.tokens = append(tokens.tokens, domain.Token{
		ID: "t2", UUID: "22222222-2222-2222-2222-222222222222", IsActive: true, ExpiresAt: now.Add(time.Hour),
	})

	if err := mgr.Sync(context.Background()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if runtimeStub.reloadCalls != 1 {
		t.Fatalf("expected one reload call, got %d", runtimeStub.reloadCalls)
	}
}
