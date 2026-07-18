package checker

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"outless/shared/vless"
)

// checkHandshake validates the transport layer before a full proxy test.
func checkHandshake(ctx context.Context, p vless.Parsed, timeout time.Duration) error {
	switch p.Network {
	case networkWS:
		return checkWS(ctx, p, timeout)
	case networkGRPC:
		if p.Security == securityTLS || p.Security == securityReality {
			return checkTLS(ctx, p, timeout)
		}
		return nil
	default:
		if p.Security == securityTLS || p.Security == securityReality {
			return checkTLS(ctx, p, timeout)
		}
		return nil
	}
}

func wsKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func checkTLS(ctx context.Context, p vless.Parsed, timeout time.Duration) error {
	sni := p.SNI
	if sni == "" {
		sni = p.Host
	}
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))

	d := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: timeout},
		Config:    &tls.Config{ServerName: sni},
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("tls handshake: %w", err)
	}
	_ = conn.Close()
	return nil
}

func checkWS(ctx context.Context, p vless.Parsed, timeout time.Duration) error {
	scheme := networkHTTP
	if p.Security == securityTLS || p.Security == securityReality {
		scheme = "https"
	}

	path := p.Path
	if path == "" {
		path = "/"
	}

	u := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
		Path:   path,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("ws request: %w", err)
	}

	host := p.HostHeader
	if host == "" {
		host = p.Host
	}
	req.Host = host
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", wsKey())
	req.Header.Set("Sec-WebSocket-Version", "13")

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if p.Security == securityTLS || p.Security == securityReality {
		sni := p.SNI
		if sni == "" {
			sni = p.Host
		}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{ServerName: sni},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ws handshake: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode == http.StatusSwitchingProtocols {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	return fmt.Errorf("ws unexpected status %d", resp.StatusCode)
}
