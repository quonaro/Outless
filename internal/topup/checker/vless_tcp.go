package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"outless/shared/vless"
)

// checkVLESSTCP performs a raw VLESS v0 handshake over plain TCP or TLS.
func checkVLESSTCP(ctx context.Context, p vless.Parsed, uid [16]byte, addr string, deadline time.Time) error {
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp dial %s: %w", addr, err)
	}
	defer conn.Close()

	c := conn

	if p.Security == securityTLS {
		sni := p.SNI
		if sni == "" {
			sni = p.Host
		}

		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         sni,
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
			NextProtos:         p.ALPN,
		})

		if err := tlsConn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("tls set deadline: %w", err)
		}

		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("tls handshake (sni=%s): %w", sni, err)
		}

		c = tlsConn
	}

	return sendVLESSRequest(c, uid, p.Flow, deadline)
}
