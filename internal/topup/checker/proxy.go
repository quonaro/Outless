package checker

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"outless/shared/vless"

	box "github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type proxyResult struct {
	Latency time.Duration
	RealIP  string
}

const proxyStartRetries = 3

// checkProxy runs an in-process sing-box instance with the parsed node as outbound
// and tests connectivity through a local mixed inbound.
func checkProxy(ctx context.Context, p vless.Parsed, timeout time.Duration) (proxyResult, error) {
	var lastErr error
	for attempt := 0; attempt < proxyStartRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return proxyResult{}, err
		}

		port, err := freePort(ctx)
		if err != nil {
			return proxyResult{}, fmt.Errorf("allocating local port: %w", err)
		}

		opts, err := buildProxyOptions(p, port)
		if err != nil {
			return proxyResult{}, fmt.Errorf("building sing-box options: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, timeout+10*time.Second)
		defer cancel()

		instance, err := box.New(box.Options{Context: ctx, Options: opts})
		if err != nil {
			return proxyResult{}, fmt.Errorf("creating sing-box: %w", err)
		}

		if err := instance.Start(); err != nil {
			_ = instance.Close()
			if attempt < proxyStartRetries-1 && errors.Is(err, syscall.EADDRINUSE) {
				lastErr = err
				continue
			}
			return proxyResult{}, fmt.Errorf("starting sing-box: %w", err)
		}

		if err := waitForPort(ctx, port, timeout); err != nil {
			_ = instance.Close()
			return proxyResult{}, err
		}

		result, err := testHTTP(ctx, port, timeout)
		_ = instance.Close()
		if err != nil {
			return proxyResult{}, err
		}
		return result, nil
	}
	return proxyResult{}, fmt.Errorf("starting sing-box after retries: %w", lastErr)
}

func buildProxyOptions(p vless.Parsed, port int) (option.Options, error) {
	vlessOut := option.VLESSOutboundOptions{
		ServerOptions: option.ServerOptions{Server: p.Host, ServerPort: uint16(p.Port)},
		UUID:          p.UUID,
		Flow:          p.Flow,
	}

	if tls := buildProxyTLS(p); tls != nil {
		vlessOut.TLS = tls
	}
	if transport := buildProxyTransport(p); transport != nil {
		vlessOut.Transport = transport
	}

	out := option.Outbound{Type: C.TypeVLESS, Tag: "out", VLESSOptions: vlessOut}
	block := option.Outbound{Type: C.TypeBlock, Tag: "block"}

	listenAddr := netip.MustParseAddr("127.0.0.1")
	inbound := option.Inbound{
		Type: C.TypeMixed,
		Tag:  "in",
		MixedOptions: option.HTTPMixedInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     option.NewListenAddress(listenAddr),
				ListenPort: uint16(port),
			},
		},
	}

	route := &option.RouteOptions{
		Rules: []option.Rule{
			{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultRule{
					Inbound:  option.Listable[string]{"in"},
					Outbound: "out",
				},
			},
		},
		Final: "block",
	}

	return option.Options{
		Log:       &option.LogOptions{Level: "warn", Timestamp: false},
		Inbounds:  []option.Inbound{inbound},
		Outbounds: []option.Outbound{out, block},
		Route:     route,
	}, nil
}

func buildProxyTLS(p vless.Parsed) *option.OutboundTLSOptions {
	switch p.Security {
	case securityReality:
		tls := &option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: p.SNI,
			Reality:    &option.OutboundRealityOptions{Enabled: true, PublicKey: p.PBK, ShortID: p.SID},
		}
		fp := p.FP
		if fp == "" {
			fp = "chrome"
		}
		tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: fp}
		return tls
	case securityTLS:
		tls := &option.OutboundTLSOptions{Enabled: true, ServerName: p.SNI}
		if len(p.ALPN) > 0 {
			tls.ALPN = p.ALPN
		}
		if p.FP != "" {
			tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: p.FP}
		}
		return tls
	default:
		return nil
	}
}

func buildProxyTransport(p vless.Parsed) *option.V2RayTransportOptions {
	switch p.Network {
	case networkWS:
		path := p.Path
		if path == "" {
			path = "/"
		}
		ws := option.V2RayWebsocketOptions{Path: path}
		if p.HostHeader != "" {
			ws.Headers = option.HTTPHeader{"Host": option.Listable[string]{p.HostHeader}}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: ws}
	case networkHTTP:
		httpOpt := option.V2RayHTTPOptions{Path: p.Path}
		if p.HostHeader != "" {
			httpOpt.Host = option.Listable[string]{p.HostHeader}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeHTTP, HTTPOptions: httpOpt}
	case networkHTTPUpgrade:
		hu := option.V2RayHTTPUpgradeOptions{Path: p.Path, Host: p.HostHeader}
		if p.HostHeader != "" {
			hu.Headers = option.HTTPHeader{"Host": option.Listable[string]{p.HostHeader}}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeHTTPUpgrade, HTTPUpgradeOptions: hu}
	case networkGRPC:
		return &option.V2RayTransportOptions{
			Type:        C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{ServiceName: p.Service},
		}
	default:
		return nil
	}
}

func freePort(ctx context.Context) (int, error) {
	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port, nil
}

func waitForPort(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for sing-box on %s", addr)
}

var probeURLs = []string{
	"https://ifconfig.me/ip",
	"https://icanhazip.com",
	"https://api.ipify.org",
}

func testHTTP(ctx context.Context, port int, timeout time.Duration) (proxyResult, error) {
	proxyURL := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{},
		},
	}

	start := time.Now()
	var lastErr error
	for _, probe := range probeURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe, nil)
		if err != nil {
			return proxyResult{}, fmt.Errorf("creating request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			lastErr = fmt.Errorf("unexpected status %d from %s", resp.StatusCode, probe)
			continue
		}

		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) == nil {
			lastErr = fmt.Errorf("invalid real ip response from %s: %q", probe, ip)
			continue
		}
		return proxyResult{Latency: time.Since(start), RealIP: ip}, nil
	}

	if lastErr != nil {
		return proxyResult{}, fmt.Errorf("proxy test: %w", lastErr)
	}
	return proxyResult{}, fmt.Errorf("proxy test: all probe URLs failed")
}
