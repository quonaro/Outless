package checker

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"outless/shared/vless"
)

// checkTCP attempts a TCP connection to the parsed VLESS endpoint.
func checkTCP(ctx context.Context, p vless.Parsed, timeout time.Duration) error {
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	d := &net.Dialer{Timeout: timeout}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp dial: %w", err)
	}
	_ = conn.Close()
	return nil
}
