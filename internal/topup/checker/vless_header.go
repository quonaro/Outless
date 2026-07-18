package checker

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// buildVLESSRequestHeader builds a VLESS v0 request header to a fixed target.
func buildVLESSRequestHeader(uid [16]byte, flow string) []byte {
	addr := []byte(vlessTargetAddr)

	var addons []byte
	if flow == vlessVisionFlow {
		flowBytes := []byte(vlessVisionFlow)
		addons = append(addons, 0x0a, byte(len(flowBytes)))
		addons = append(addons, flowBytes...)
	}

	buf := make([]byte, 0, 1+16+1+len(addons)+1+2+1+1+len(addr))
	buf = append(buf, vlessProtocolVersion)
	buf = append(buf, uid[:]...)
	buf = append(buf, byte(len(addons)))
	buf = append(buf, addons...)
	buf = append(buf, vlessCommandTCP)

	var port [2]byte
	binary.BigEndian.PutUint16(port[:], vlessTargetPort)
	buf = append(buf, port[:]...)

	buf = append(buf, vlessAddrTypeDomain)
	buf = append(buf, byte(len(addr)))
	buf = append(buf, addr...)

	return buf
}

// sendVLESSRequest writes a VLESS request header and validates the response.
func sendVLESSRequest(c net.Conn, uid [16]byte, flow string, deadline time.Time) error {
	if err := c.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}
	if _, err := c.Write(buildVLESSRequestHeader(uid, flow)); err != nil {
		return fmt.Errorf("write vless request: %w", err)
	}
	return readVLESSResponse(c, deadline)
}

// readVLESSResponse reads and validates the VLESS response header.
func readVLESSResponse(c net.Conn, deadline time.Time) error {
	if err := c.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(c, resp); err != nil {
		return fmt.Errorf("read vless response: %w", err)
	}

	if resp[0] != vlessProtocolVersion {
		return fmt.Errorf("unexpected vless response version: %x", resp[0])
	}

	if resp[1] != 0 {
		addons := make([]byte, resp[1])
		if _, err := io.ReadFull(c, addons); err != nil {
			return fmt.Errorf("read vless response addons: %w", err)
		}
	}

	return nil
}
