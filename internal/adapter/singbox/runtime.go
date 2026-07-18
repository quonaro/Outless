package singbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"outless/internal/domain"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
)

// Compile-time check that RuntimeController implements domain.RuntimeController.
var _ domain.RuntimeController = (*RuntimeController)(nil)

// RuntimeController manages an embedded sing-box instance. Because sing-box has
// no in-place graceful reload, Reload performs a debounced close+recreate.
type RuntimeController struct {
	logger          *slog.Logger
	tokenRepo       domain.TokenRepository
	nodeRepo        domain.NodeRepository
	inboundRepo     domain.InboundRepository
	singboxLogLevel string
	debounce        time.Duration
	logOutput       func(string)

	mu             sync.Mutex
	instance       *box.Box
	trafficManager *trafficontrol.Manager
	baseCtx        context.Context
	timer          *time.Timer
	started        bool
	inboundErrors  map[string]string
}

// NewRuntimeController creates an embedded sing-box runtime controller.
func NewRuntimeController(
	logger *slog.Logger,
	tokenRepo domain.TokenRepository,
	nodeRepo domain.NodeRepository,
	inboundRepo domain.InboundRepository,
	singboxLogLevel string,
	debounce time.Duration,
	logOutput func(string),
) *RuntimeController {
	if debounce <= 0 {
		debounce = 3 * time.Second
	}
	return &RuntimeController{
		logger:          logger,
		tokenRepo:       tokenRepo,
		nodeRepo:        nodeRepo,
		inboundRepo:     inboundRepo,
		singboxLogLevel: singboxLogLevel,
		debounce:        debounce,
		logOutput:       logOutput,
		inboundErrors:   make(map[string]string),
	}
}

// Description identifies this controller.
func (r *RuntimeController) Description() string { return "embedded-singbox" }

// Start builds the initial config from the database and starts sing-box.
func (r *RuntimeController) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.baseCtx = ctx
	if err := r.rebuildLocked(ctx); err != nil {
		return fmt.Errorf("starting sing-box: %w", err)
	}
	r.started = true
	r.logger.Info("sing-box started")
	return nil
}

// Reload schedules a debounced rebuild of the sing-box instance.
func (r *RuntimeController) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return nil
	}
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(r.debounce, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.started || r.baseCtx == nil {
			return
		}
		if err := r.rebuildLocked(r.baseCtx); err != nil {
			r.logger.Error("sing-box reload failed", slog.String("error", err.Error()))
		} else {
			r.logger.Info("sing-box reloaded")
		}
	})
	return nil
}

// ForceSync rebuilds the instance immediately, bypassing debounce.
func (r *RuntimeController) ForceSync() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started || r.baseCtx == nil {
		return nil
	}
	if err := r.rebuildLocked(r.baseCtx); err != nil {
		return fmt.Errorf("force sync sing-box: %w", err)
	}
	r.logger.Info("sing-box force-synced")
	return nil
}

// RemoveUser triggers a reload; sing-box has no granular user removal API.
func (r *RuntimeController) RemoveUser(string) error { return r.Reload() }

// RemoveRulesForUser triggers a reload; rules are regenerated from the database.
func (r *RuntimeController) RemoveRulesForUser(string) error { return r.Reload() }

// TrafficSnapshot returns a point-in-time view of current traffic counters.
func (r *RuntimeController) TrafficSnapshot() *domain.TrafficSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.trafficManager == nil {
		return nil
	}

	snap := r.trafficManager.Snapshot()
	connections := make([]domain.TrafficConnection, 0, len(snap.Connections))
	for _, conn := range snap.Connections {
		meta := conn.Metadata()
		connDomain := meta.Metadata.Domain
		if connDomain == "" {
			connDomain = meta.Metadata.Destination.Fqdn
		}
		connections = append(connections, domain.TrafficConnection{
			ID:       meta.ID.String(),
			User:     meta.Metadata.User,
			NodeID:   parseNodeID(meta.Metadata.User),
			Inbound:  meta.Metadata.Inbound,
			Domain:   connDomain,
			SourceIP: meta.Metadata.Source.Addr.String(),
			Upload:   meta.Upload.Load(),
			Download: meta.Download.Load(),
		})
	}

	return &domain.TrafficSnapshot{
		UploadTotal:   snap.Upload,
		DownloadTotal: snap.Download,
		Connections:   connections,
	}
}

// Stop closes the running sing-box instance.
func (r *RuntimeController) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
	r.closeLocked()
	r.started = false
	r.logger.Info("sing-box stopped")
}

// rebuildLocked regenerates options from the database and replaces the running
// instance. Caller must hold r.mu.
func (r *RuntimeController) rebuildLocked(ctx context.Context) error {
	tokens, nodes, inbounds, err := r.loadRuntimeData(ctx)
	if err != nil {
		return err
	}

	activeInbounds := make([]domain.Inbound, len(inbounds))
	copy(activeInbounds, inbounds)
	r.inboundErrors = make(map[string]string)

	for {
		instance, retry, err := r.buildAndStart(ctx, tokens, nodes, activeInbounds)
		if err != nil {
			return err
		}
		if retry != nil {
			failed := activeInbounds[retry.idx]
			r.inboundErrors[failed.ID] = retry.err.Error()
			if r.logger != nil {
				r.logger.Warn("skipping inbound due to bind error",
					slog.String("tag", retry.tag),
					slog.String("id", failed.ID),
					slog.String("address", failed.Address),
					slog.Int("port", failed.Port),
					slog.String("error", retry.err.Error()))
			}
			activeInbounds = append(activeInbounds[:retry.idx], activeInbounds[retry.idx+1:]...)
			continue
		}

		if clashSrv, ok := instance.Router().ClashServer().(*clashapi.Server); ok && clashSrv != nil {
			r.trafficManager = clashSrv.TrafficManager()
		}

		r.instance = instance
		if len(activeInbounds) < len(inbounds) && r.logger != nil {
			r.logger.Warn("sing-box started with skipped inbounds",
				slog.Int("total", len(inbounds)),
				slog.Int("active", len(activeInbounds)),
				slog.Int("skipped", len(inbounds)-len(activeInbounds)))
		}
		return nil
	}
}

// bindRetry instructs rebuildLocked to remove one failing inbound and retry.
type bindRetry struct {
	idx int
	tag string
	err error
}

func (r *RuntimeController) loadRuntimeData(ctx context.Context) (
	tokens []domain.Token,
	nodes []domain.Node,
	inbounds []domain.Inbound,
	err error,
) {
	now := time.Now().UTC()

	tokens, err = r.tokenRepo.ListActive(ctx, now)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("listing active tokens: %w", err)
	}
	nodes, err = r.nodeRepo.List(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("listing nodes: %w", err)
	}
	inbounds, err = r.inboundRepo.List(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("listing inbounds: %w", err)
	}
	return tokens, nodes, inbounds, nil
}

func toHubInboundConfig(inbound domain.Inbound) HubInboundConfig {
	return HubInboundConfig{
		Listen:     inbound.Address,
		Port:       inbound.Port,
		SNI:        inbound.SNI,
		Handshake:  inbound.Handshake,
		PrivateKey: inbound.PrivateKey,
		ShortID:    inbound.ShortID,
	}
}

func (r *RuntimeController) buildAndStart(
	ctx context.Context,
	tokens []domain.Token,
	nodes []domain.Node,
	activeInbounds []domain.Inbound,
) (*box.Box, *bindRetry, error) {
	hubInbounds := make([]HubInboundConfig, 0, len(activeInbounds))
	for _, inbound := range activeInbounds {
		hubInbounds = append(hubInbounds, toHubInboundConfig(inbound))
	}

	opts, err := GenerateOptions(tokens, nodes, hubInbounds, r.singboxLogLevel, r.logger)
	if err != nil {
		return nil, nil, fmt.Errorf("generating sing-box options: %w", err)
	}

	r.closeLocked()

	if r.logger != nil && r.logger.Enabled(ctx, slog.LevelDebug) {
		debugJSON, _ := json.MarshalIndent(opts, "", "  ")
		r.logger.Debug("sing-box options", slog.String("config", string(debugJSON)))
	}

	instance, err := box.New(box.Options{
		Context:           ctx,
		Options:           opts,
		PlatformLogWriter: newSingBoxLogWriter(r.logOutput),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("creating sing-box instance: %w", err)
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		retry, retryErr := r.newBindRetry(err, len(activeInbounds))
		return nil, retry, retryErr
	}

	return instance, nil, nil
}

func (r *RuntimeController) newBindRetry(err error, activeCount int) (*bindRetry, error) {
	if !isBindError(err) {
		return nil, fmt.Errorf("starting sing-box instance: %w", err)
	}
	tag := failedInboundTag(err.Error())
	idx := inboundIndexFromTag(tag)
	if idx < 0 || idx >= activeCount {
		return nil, fmt.Errorf("starting sing-box instance: %w", err)
	}
	return &bindRetry{idx: idx, tag: tag, err: err}, nil
}

// isBindError reports whether an error is a TCP bind failure such as permission
// denied or address already in use.
func isBindError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "bind:") && (strings.Contains(msg, "permission denied") || strings.Contains(msg, "address already in use"))
}

// failedInboundTag extracts the vless-in-N tag from a sing-box start error.
func failedInboundTag(errMsg string) string {
	const prefix = "initialize inbound/vless["
	idx := strings.Index(errMsg, prefix)
	if idx == -1 {
		return ""
	}
	rest := errMsg[idx+len(prefix):]
	end := strings.Index(rest, "]")
	if end == -1 {
		return ""
	}
	return rest[:end]
}

// inboundIndexFromTag parses the numeric suffix of a vless-in-N tag.
func inboundIndexFromTag(tag string) int {
	parts := strings.Split(tag, "-")
	if len(parts) == 0 {
		return -1
	}
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return -1
	}
	return n
}

// InboundStatus returns the runtime status for an inbound: "disabled" with a
// reason if it was skipped during the last rebuild, otherwise "active".
func (r *RuntimeController) InboundStatus(id string) (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if reason, ok := r.inboundErrors[id]; ok {
		return "disabled", reason
	}
	return "active", ""
}

func (r *RuntimeController) closeLocked() {
	if r.instance != nil {
		_ = r.instance.Close()
		r.instance = nil
	}
	r.trafficManager = nil
}

// parseNodeID extracts node identifier from a sing-box inbound user name.
// Expected format: "t-<tokenID>-n-<nodeID>".
func parseNodeID(user string) string {
	parts := strings.Split(user, "-")
	if len(parts) < 4 || parts[0] != "t" || parts[2] != "n" {
		return ""
	}
	return parts[3]
}
