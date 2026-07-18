package checker

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/crypto/hkdf"

	"outless/shared/vless"
)

func checkVLESSReality(ctx context.Context, p vless.Parsed, uid [16]byte, addr string, deadline time.Time) error {
	rc, err := buildRealityConfig(p)
	if err != nil {
		return err
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp dial %s: %w", addr, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if err := validateVlessOverReality(ctx, conn, uid, rc, deadline); err != nil {
		return fmt.Errorf("reality handshake: %w", err)
	}

	return nil
}

func buildRealityConfig(p vless.Parsed) (*realityConfig, error) {
	if p.PBK == "" {
		return nil, fmt.Errorf("reality requires pbk parameter")
	}

	publicKey, err := decodeRealityPublicKey(p.PBK)
	if err != nil {
		return nil, fmt.Errorf("invalid pbk: %w", err)
	}

	shortID := make([]byte, 8)
	if p.SID != "" {
		sidBytes, err := hex.DecodeString(p.SID)
		if err != nil {
			return nil, fmt.Errorf("invalid sid: %w", err)
		}
		copy(shortID, sidBytes)
	}

	serverName := p.SNI
	if serverName == "" {
		serverName = p.Host
	}

	fp := p.FP
	if fp == "" {
		fp = "chrome"
	}

	return &realityConfig{
		PublicKey:   publicKey,
		ShortId:     shortID,
		Fingerprint: fp,
		ServerName:  serverName,
		Flow:        p.Flow,
	}, nil
}

func decodeRealityPublicKey(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err == nil && len(b) == 32 {
		return b, nil
	}

	b, err = base64.URLEncoding.DecodeString(s)
	if err == nil && len(b) == 32 {
		return b, nil
	}

	b, err = base64.StdEncoding.DecodeString(s)
	if err == nil && len(b) == 32 {
		return b, nil
	}

	return nil, fmt.Errorf("invalid public key length %d", len(b))
}

type realityConfig struct {
	PublicKey   []byte
	ShortId     []byte
	Fingerprint string
	ServerName  string
	Flow        string
}

func validateVlessOverReality(ctx context.Context, conn net.Conn, uid [16]byte, rc *realityConfig, deadline time.Time) error {
	var authKey []byte

	utlsConfig := realityUTLSConfig(rc, &authKey)
	fp := realityFingerprint(rc.Fingerprint)
	uConn := utls.UClient(conn, utlsConfig, *fp)

	var err error
	authKey, err = prepareRealityAuthKey(uConn, rc)
	if err != nil {
		return fmt.Errorf("prepare hello: %w", err)
	}

	if err := uConn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set handshake deadline: %w", err)
	}

	if err := uConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("handshake: %w", err)
	}

	if err := uConn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	if _, err := uConn.Write(buildVLESSRequestHeader(uid, rc.Flow)); err != nil {
		return fmt.Errorf("write request header: %w", err)
	}

	if rc.Flow == vlessVisionFlow {
		if err := writeVisionPadding(uConn, deadline); err != nil {
			return err
		}
	}

	return readVLESSResponse(uConn, deadline)
}

func realityUTLSConfig(rc *realityConfig, authKey *[]byte) *utls.Config {
	return &utls.Config{
		ServerName:             rc.ServerName,
		InsecureSkipVerify:     true,
		SessionTicketsDisabled: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no certificates")
			}

			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}

			if pub, ok := cert.PublicKey.(ed25519.PublicKey); ok {
				h := hmac.New(sha512.New, *authKey)
				h.Write(pub)
				if hmac.Equal(h.Sum(nil), cert.Signature) {
					return nil
				}
			}

			opts := x509.VerifyOptions{
				DNSName:       rc.ServerName,
				Intermediates: x509.NewCertPool(),
			}
			for _, raw := range rawCerts[1:] {
				if c, err := x509.ParseCertificate(raw); err == nil {
					opts.Intermediates.AddCert(c)
				}
			}

			_, err = cert.Verify(opts)
			return err
		},
	}
}

func prepareRealityAuthKey(uConn *utls.UConn, rc *realityConfig) ([]byte, error) {
	if err := uConn.BuildHandshakeState(); err != nil {
		return nil, fmt.Errorf("build handshake state: %w", err)
	}

	hello := uConn.HandshakeState.Hello
	hello.SessionId = make([]byte, 32)
	copy(hello.Raw[39:], hello.SessionId)
	hello.SessionId[0] = versionX
	hello.SessionId[1] = versionY
	hello.SessionId[2] = versionZ
	hello.SessionId[3] = 0
	binary.BigEndian.PutUint32(hello.SessionId[4:], uint32(time.Now().Unix()))
	copy(hello.SessionId[8:], rc.ShortId)

	publicKey, err := ecdh.X25519().NewPublicKey(rc.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	ecdhe := uConn.HandshakeState.State13.KeyShareKeys.Ecdhe
	if ecdhe == nil {
		ecdhe = uConn.HandshakeState.State13.KeyShareKeys.MlkemEcdhe
	}
	if ecdhe == nil {
		return nil, fmt.Errorf("fingerprint %q does not support TLS 1.3", rc.Fingerprint)
	}

	authKey, err := ecdhe.ECDH(publicKey)
	if err != nil {
		return nil, fmt.Errorf("ECDH: %w", err)
	}

	if _, err := hkdf.New(sha256.New, authKey, hello.Random[:20], []byte("REALITY")).Read(authKey); err != nil {
		return nil, fmt.Errorf("HKDF: %w", err)
	}

	if err := encryptRealitySessionID(authKey, hello); err != nil {
		return nil, err
	}

	return authKey, nil
}

func encryptRealitySessionID(authKey []byte, hello *utls.PubClientHelloMsg) error {
	block, err := aes.NewCipher(authKey)
	if err != nil {
		return fmt.Errorf("AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("GCM: %w", err)
	}

	aead.Seal(hello.SessionId[:0], hello.Random[20:], hello.SessionId[:16], hello.Raw)
	copy(hello.Raw[39:], hello.SessionId)

	return nil
}

func writeVisionPadding(w io.Writer, deadline time.Time) error {
	var padLen [1]byte
	if _, err := rand.Read(padLen[:]); err != nil {
		return fmt.Errorf("padding: %w", err)
	}

	if c, ok := w.(net.Conn); ok {
		if err := c.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("set write deadline: %w", err)
		}
	}

	pad := make([]byte, 5+int(padLen[0]))
	pad[0] = 0x00 // command = continue
	pad[1] = 0x00
	pad[2] = 0x00
	pad[3] = 0
	pad[4] = padLen[0]

	if _, err := w.Write(pad); err != nil {
		return fmt.Errorf("write padding: %w", err)
	}

	return nil
}

func realityFingerprint(name string) *utls.ClientHelloID {
	switch strings.ToLower(name) {
	case "firefox":
		return &utls.HelloFirefox_Auto
	case "safari":
		return &utls.HelloSafari_Auto
	case "edge":
		return &utls.HelloEdge_Auto
	case "ios":
		return &utls.HelloIOS_Auto
	case "android":
		return &utls.HelloAndroid_11_OkHttp
	case "random", "randomized":
		return &utls.HelloRandomized
	default:
		return &utls.HelloChrome_Auto
	}
}

const (
	versionX = 26
	versionY = 7
	versionZ = 11
)
