package checker

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/google/uuid"

	"outless/shared/vless"
)

const (
	vlessProtocolVersion = 0x00
	vlessCommandTCP      = 0x01
	vlessAddrTypeDomain  = 0x02
	vlessTargetPort      = uint16(80)
	vlessTargetAddr      = "google.com"
	vlessVisionFlow      = "xtls-rprx-vision"
)

// checkVLESS performs a raw VLESS v0 handshake without starting a local inbound.
// It supports tcp (none/tls/reality) and ws transports.
func checkVLESS(ctx context.Context, p vless.Parsed, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	parsedUUID, err := uuid.Parse(p.UUID)
	if err != nil {
		return fmt.Errorf("parse uuid: %w", err)
	}
	uid := [16]byte(parsedUUID)

	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))

	switch p.Network {
	case networkTCP, "":
		switch p.Security {
		case securityReality:
			return checkVLESSReality(ctx, p, uid, addr, deadline)
		case securityTLS, securityNone:
			return checkVLESSTCP(ctx, p, uid, addr, deadline)
		default:
			return fmt.Errorf("unsupported security %q for vless tcp handshake", p.Security)
		}
	case networkWS:
		return checkVLESSWS(ctx, p, uid, addr, deadline)
	default:
		return fmt.Errorf("transport %q not supported by vless handshake", p.Network)
	}
}
