package checker

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"outless/shared/vless"
)

const (
	wsGUID         = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	maxWSFrameSize = 64 * 1024
)

// checkVLESSWS upgrades the connection to WebSocket and performs a raw VLESS v0 handshake.
func checkVLESSWS(ctx context.Context, p vless.Parsed, uid [16]byte, addr string, deadline time.Time) error {
	conn, err := dialVLESSWS(ctx, p, addr, deadline)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := wsUpgrade(conn, vlessWSHost(p), vlessWSPath(p), deadline); err != nil {
		return err
	}

	return sendVLESSWSFrame(conn, uid, p.Flow, deadline)
}

func dialVLESSWS(ctx context.Context, p vless.Parsed, addr string, deadline time.Time) (net.Conn, error) {
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial %s: %w", addr, err)
	}

	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	if p.Security != securityTLS && p.Security != securityReality {
		return conn, nil
	}

	sni := p.SNI
	if sni == "" {
		sni = p.Host
	}

	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	})

	if err := tlsConn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("tls set deadline: %w", err)
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("tls handshake (sni=%s): %w", sni, err)
	}

	return tlsConn, nil
}

func vlessWSHost(p vless.Parsed) string {
	if p.HostHeader != "" {
		return p.HostHeader
	}
	if p.SNI != "" {
		return p.SNI
	}
	return p.Host
}

func vlessWSPath(p vless.Parsed) string {
	if p.Path != "" {
		return p.Path
	}
	return "/"
}

func sendVLESSWSFrame(conn net.Conn, uid [16]byte, flow string, deadline time.Time) error {
	payload := buildVLESSRequestHeader(uid, flow)
	frame, err := buildMaskedWSFrame(payload)
	if err != nil {
		return fmt.Errorf("build ws frame: %w", err)
	}

	if err := conn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	if _, err := conn.Write(frame); err != nil {
		return fmt.Errorf("ws write vless: %w", err)
	}

	br := bufio.NewReader(conn)
	if err := conn.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	opcode, resp, err := readWSFrame(br, conn, deadline)
	if err != nil {
		return fmt.Errorf("ws read vless response: %w", err)
	}

	if opcode == 8 {
		return fmt.Errorf("ws close frame received")
	}

	if len(resp) < 2 || resp[0] != vlessProtocolVersion || resp[1] != 0 {
		return fmt.Errorf("unexpected vless response: %x", resp)
	}

	return nil
}

func wsUpgrade(c net.Conn, host, path string, deadline time.Time) error {
	wsKey, err := generateWSKey()
	if err != nil {
		return fmt.Errorf("generate ws key: %w", err)
	}

	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, host, wsKey,
	)

	if err := c.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	if _, err := io.WriteString(c, req); err != nil {
		return fmt.Errorf("ws upgrade write: %w", err)
	}

	if err := c.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	br := bufio.NewReader(c)
	tp := textproto.NewReader(br)

	statusLine, err := tp.ReadLine()
	if err != nil {
		return fmt.Errorf("ws read status: %w", err)
	}

	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return fmt.Errorf("invalid status line: %s", statusLine)
	}

	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid status code: %w", err)
	}

	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return fmt.Errorf("ws read headers: %w", err)
	}

	if code != 101 {
		return fmt.Errorf("ws upgrade failed: %s", statusLine)
	}

	expectedAccept := websocketAccept(wsKey)
	if got := headers.Get("Sec-WebSocket-Accept"); got != "" && got != expectedAccept {
		return fmt.Errorf("ws accept mismatch: got %s, want %s", got, expectedAccept)
	}

	return nil
}

func generateWSKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

func websocketAccept(key string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(key))
	_, _ = h.Write([]byte(wsGUID))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func buildMaskedWSFrame(payload []byte) ([]byte, error) {
	if len(payload) > 125 {
		return nil, fmt.Errorf("payload length %d too large for simple ws frame", len(payload))
	}

	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return nil, err
	}

	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}

	frame := []byte{0x82, 0x80 | byte(len(payload))}
	frame = append(frame, mask...)
	frame = append(frame, masked...)

	return frame, nil
}

func readWSFrame(r io.Reader, c net.Conn, deadline time.Time) (opcode byte, payload []byte, err error) {
	if err := c.SetReadDeadline(deadline); err != nil {
		return 0, nil, fmt.Errorf("set read deadline: %w", err)
	}

	hdr := make([]byte, 2)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, fmt.Errorf("read ws header: %w", err)
	}

	opcode = hdr[0] & 0x0f
	masked := hdr[1]&0x80 != 0
	length := uint64(hdr[1] & 0x7f)

	switch length {
	case 126:
		b := make([]byte, 2)
		if _, err := io.ReadFull(r, b); err != nil {
			return 0, nil, fmt.Errorf("read ws length: %w", err)
		}
		length = uint64(binary.BigEndian.Uint16(b))
	case 127:
		b := make([]byte, 8)
		if _, err := io.ReadFull(r, b); err != nil {
			return 0, nil, fmt.Errorf("read ws length: %w", err)
		}
		length = binary.BigEndian.Uint64(b)
	}

	if length > maxWSFrameSize {
		return 0, nil, fmt.Errorf("ws frame too large: %d", length)
	}

	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(r, mask); err != nil {
			return 0, nil, fmt.Errorf("read ws mask: %w", err)
		}
	}

	payload = make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, fmt.Errorf("read ws payload: %w", err)
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}
