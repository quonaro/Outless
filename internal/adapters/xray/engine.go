package xray

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	vlesspkg "outless/pkg/vless"

	handlercmd "github.com/xtls/xray-core/app/proxyman/command"
	approuter "github.com/xtls/xray-core/app/router"
	routercmd "github.com/xtls/xray-core/app/router/command"
	xcore "github.com/xtls/xray-core/core"
	xrayconf "github.com/xtls/xray-core/infra/conf"
	"golang.org/x/net/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const defaultSocksInboundTag = "socks-in"

const (
	maxRetries     = 3
	initialBackoff = 100 * time.Millisecond
	maxBackoff     = 2 * time.Second
)

// retryWithBackoff executes fn with exponential backoff on retryable errors.
func retryWithBackoff(ctx context.Context, logger *slog.Logger, operation string, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * initialBackoff
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable (gRPC errors, temporary network errors)
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded {
				logger.Debug("retryable error, backing off", slog.String("operation", operation), slog.Int("attempt", attempt+1), slog.String("error", err.Error()))
				continue
			}
		}

		// For non-retryable errors, return immediately
		break
	}
	return lastErr
}

// Engine probes nodes via native Xray gRPC API: temporary outbound + routing rule, then HTTP GET through local SOCKS.
type Engine struct {
	logger       *slog.Logger
	probeURL     string
	grpcTarget   string
	socksAddr    string
	socksInTag   string
	probeTimeout time.Duration
	countryByIP  countryResolver

	mu   sync.Mutex
	conn *grpc.ClientConn
	hs   handlercmd.HandlerServiceClient
	rs   routercmd.RoutingServiceClient
}

// GeoIPConfig controls local MMDB country lookup and optional auto-update.
type GeoIPConfig struct {
	DBPath string
	DBURL  string
	Auto   bool
	TTL    time.Duration
}

// NewEngine constructs an Xray-backed proxy engine using gRPC HandlerService + RoutingService.
// adminURL is the Xray API address (e.g. http://127.0.0.1:10085); only host and port are used for gRPC.
// socksAddr is the local SOCKS inbound (e.g. 127.0.0.1:1080) used to perform the HTTP probe through Xray.
func NewEngine(logger *slog.Logger, probeURL, adminURL, socksAddr string, geoIP GeoIPConfig, probeTimeout time.Duration) *Engine {
	if probeTimeout <= 0 {
		probeTimeout = 10 * time.Second
	}
	if strings.TrimSpace(socksAddr) == "" {
		socksAddr = "127.0.0.1:1080"
	}
	target, err := parseGRPCTarget(adminURL)
	if err != nil {
		logger.Warn("invalid xray admin_url for gRPC; probes will fail until fixed", slog.String("admin_url", adminURL), slog.String("error", err.Error()))
		target = ""
	}
	return &Engine{
		logger:       logger,
		probeURL:     probeURL,
		grpcTarget:   target,
		socksAddr:    socksAddr,
		socksInTag:   defaultSocksInboundTag,
		probeTimeout: probeTimeout,
		countryByIP:  newGeoIPCountryResolver(logger, geoIP),
	}
}

func (e *Engine) ensureConn(ctx context.Context) error {
	if e.conn != nil {
		return nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, e.grpcTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("grpc dial %s: %w", e.grpcTarget, err)
	}
	e.conn = conn
	e.hs = handlercmd.NewHandlerServiceClient(conn)
	e.rs = routercmd.NewRoutingServiceClient(conn)
	return nil
}

func (e *Engine) socksHTTPClient() (*http.Client, error) {
	base := &net.Dialer{Timeout: e.probeTimeout, KeepAlive: 30 * time.Second}
	socksDialer, err := proxy.SOCKS5("tcp", e.socksAddr, nil, base)
	if err != nil {
		return nil, fmt.Errorf("socks5 dialer: %w", err)
	}

	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if d, ok := socksDialer.(proxy.ContextDialer); ok {
				return d.DialContext(ctx, network, addr)
			}
			return socksDialer.Dial(network, addr)
		},
		ForceAttemptHTTP2:     false,
		MaxIdleConnsPerHost:   -1,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: e.probeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: tr,
		Timeout:   e.probeTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func buildVLESSOutboundConfig(tag string, p vlesspkg.Parsed) (*xcore.OutboundHandlerConfig, error) {
	settingsObj := map[string]any{
		"vnext": []any{
			map[string]any{
				"address": p.Host,
				"port":    p.Port,
				"users": []any{
					map[string]any{
						"id":         p.UUID,
						"encryption": p.Encryption,
						"flow":       p.Flow,
					},
				},
			},
		},
	}
	body := map[string]any{
		"tag":            tag,
		"protocol":       "vless",
		"settings":       settingsObj,
		"streamSettings": p.StreamSettings(),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal outbound json: %w", err)
	}
	var detour xrayconf.OutboundDetourConfig
	if err := json.Unmarshal(raw, &detour); err != nil {
		return nil, fmt.Errorf("unmarshal outbound detour: %w", err)
	}
	ohc, err := detour.Build()
	if err != nil {
		return nil, fmt.Errorf("build outbound handler: %w", err)
	}
	return ohc, nil
}

func buildProbeRoutingConfig(ruleTag, socksInboundTag, probeHost, outboundTag string) (*approuter.Config, error) {
	ruleObj := map[string]any{
		"ruleTag":     ruleTag,
		"type":        "field",
		"inboundTag":  []string{socksInboundTag},
		"domain":      []string{"full:" + probeHost},
		"outboundTag": outboundTag,
	}
	ruleBytes, err := json.Marshal(ruleObj)
	if err != nil {
		return nil, fmt.Errorf("marshal routing rule: %w", err)
	}
	rc := xrayconf.RouterConfig{
		RuleList: []json.RawMessage{ruleBytes},
	}
	cfg, err := rc.Build()
	if err != nil {
		return nil, fmt.Errorf("build router config: %w", err)
	}
	return cfg, nil
}

func randomSuffix() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// parseGRPCTarget extracts host:port for gRPC from admin URL (http/https optional).
func parseGRPCTarget(adminURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(adminURL))
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("empty host")
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(host, port), nil
}
