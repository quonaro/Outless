package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/pquerna/otp/totp"
)

// TOTPService handles TOTP generation and verification.
type TOTPService struct{}

// NewTOTPService creates a new TOTP service.
func NewTOTPService() *TOTPService {
	return &TOTPService{}
}

// GenerateKey creates a new TOTP secret and provisioning URI.
func (s *TOTPService) GenerateKey(issuer, accountName string) (secret string, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", "", fmt.Errorf("generating totp key: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateCode verifies a TOTP code against a secret.
func (s *TOTPService) ValidateCode(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}
	return totp.Validate(code, secret)
}

// GenerateQRCodePNG generates a base64-encoded PNG QR code from an otpauth URI.
func (s *TOTPService) GenerateQRCodePNG(uri string) (string, error) {
	qrCode, err := qr.Encode(uri, qr.M, qr.Auto)
	if err != nil {
		return "", fmt.Errorf("encoding qr: %w", err)
	}
	qrCode, err = barcode.Scale(qrCode, 256, 256)
	if err != nil {
		return "", fmt.Errorf("scaling qr: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, qrCode); err != nil {
		return "", fmt.Errorf("encoding png: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
