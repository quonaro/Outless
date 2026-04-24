package checker

import (
	"context"
	"testing"
	"time"

	"iter"

	"outless/internal/domain"
)

type probeJobRepoStub struct {
	claimed       []domain.ProbeJob
	markSucceeded []string
	markFailed    []string
}

func (s *probeJobRepoStub) EnqueueNode(context.Context, domain.ProbeJobCreate) (domain.ProbeJob, error) {
	return domain.ProbeJob{}, nil
}
func (s *probeJobRepoStub) EnqueueBatch(context.Context, []domain.ProbeJobCreate) ([]domain.ProbeJob, error) {
	return nil, nil
}
func (s *probeJobRepoStub) ClaimPending(context.Context, int) ([]domain.ProbeJob, error) {
	return append([]domain.ProbeJob(nil), s.claimed...), nil
}
func (s *probeJobRepoStub) MarkSucceeded(_ context.Context, id string) error {
	s.markSucceeded = append(s.markSucceeded, id)
	return nil
}
func (s *probeJobRepoStub) MarkFailed(_ context.Context, id string, _ string) error {
	s.markFailed = append(s.markFailed, id)
	return nil
}
func (s *probeJobRepoStub) GetByID(context.Context, string) (domain.ProbeJob, error) {
	return domain.ProbeJob{}, nil
}
func (s *probeJobRepoStub) List(context.Context, domain.ProbeJobListFilter) ([]domain.ProbeJob, error) {
	return nil, nil
}

type nodeRepoForRunnerStub struct {
	nodes         map[string]domain.Node
	updateResults []domain.ProbeResult
}

func (s *nodeRepoForRunnerStub) IterateNodes(context.Context) iter.Seq2[domain.Node, error] {
	return func(func(domain.Node, error) bool) {}
}
func (s *nodeRepoForRunnerStub) ListVLESSURLs(context.Context, string) ([]string, error) {
	return nil, nil
}
func (s *nodeRepoForRunnerStub) UpdateProbeResult(_ context.Context, result domain.ProbeResult) error {
	s.updateResults = append(s.updateResults, result)
	return nil
}
func (s *nodeRepoForRunnerStub) Create(context.Context, domain.Node) error { return nil }
func (s *nodeRepoForRunnerStub) CreateIfAbsent(context.Context, domain.Node) (bool, error) {
	return false, nil
}
func (s *nodeRepoForRunnerStub) BulkCreateIfAbsent(context.Context, []domain.Node) ([]string, error) {
	return nil, nil
}
func (s *nodeRepoForRunnerStub) Upsert(context.Context, domain.Node) error { return nil }
func (s *nodeRepoForRunnerStub) FindByID(_ context.Context, id string) (domain.Node, error) {
	n, ok := s.nodes[id]
	if !ok {
		return domain.Node{}, domain.ErrNodeNotFound
	}
	return n, nil
}
func (s *nodeRepoForRunnerStub) List(context.Context) ([]domain.Node, error) { return nil, nil }
func (s *nodeRepoForRunnerStub) ListPage(context.Context, int, int) ([]domain.Node, error) {
	return nil, nil
}
func (s *nodeRepoForRunnerStub) ListPageByGroup(context.Context, string, int, int) ([]domain.Node, error) {
	return nil, nil
}
func (s *nodeRepoForRunnerStub) ListByGroup(context.Context, string) ([]domain.Node, error) {
	return nil, nil
}
func (s *nodeRepoForRunnerStub) ListNonHealthyByGroup(context.Context, string) ([]domain.Node, error) {
	return nil, nil
}
func (s *nodeRepoForRunnerStub) DeleteUnavailableByGroup(context.Context, string) (int64, error) {
	return 0, nil
}
func (s *nodeRepoForRunnerStub) Update(context.Context, domain.Node) error { return nil }
func (s *nodeRepoForRunnerStub) Delete(context.Context, string) error      { return nil }

type probeEngineStub struct{}

func (probeEngineStub) ProbeNode(_ context.Context, node domain.Node) (domain.ProbeResult, error) {
	return domain.ProbeResult{
		NodeID:    node.ID,
		Status:    domain.NodeStatusHealthy,
		Latency:   12 * time.Millisecond,
		Country:   "US",
		CheckedAt: time.Now().UTC(),
	}, nil
}

func TestJobRunner_RunPendingMarksSucceeded(t *testing.T) {
	t.Parallel()

	jobs := &probeJobRepoStub{
		claimed: []domain.ProbeJob{
			{
				ID:       "job-1",
				NodeID:   "node-1",
				Mode:     domain.ProbeModeFast,
				ProbeURL: "https://example.com/generate_204",
				Status:   domain.ProbeJobStatusRunning,
			},
		},
	}
	nodes := &nodeRepoForRunnerStub{
		nodes: map[string]domain.Node{
			"node-1": {ID: "node-1", URL: "vless://11111111-1111-1111-1111-111111111111@example.com:443"},
		},
	}

	runner := NewJobRunner(jobs, nodes, probeEngineStub{}, nil)
	if err := runner.RunPending(context.Background(), 10, 2); err != nil {
		t.Fatalf("run pending: %v", err)
	}

	if len(jobs.markSucceeded) != 1 || jobs.markSucceeded[0] != "job-1" {
		t.Fatalf("unexpected succeeded marks: %#v", jobs.markSucceeded)
	}
	if len(nodes.updateResults) != 1 || nodes.updateResults[0].NodeID != "node-1" {
		t.Fatalf("unexpected probe results: %#v", nodes.updateResults)
	}
}
