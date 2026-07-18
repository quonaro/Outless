package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// DeriveKey derives a 32-byte AES-256 key from a secret using HKDF-SHA256.
// The same secret will always produce the same derived key.
func DeriveKey(secret string) [32]byte {
	var key [32]byte
	derived, err := hkdf.Key(sha256.New, []byte(secret), nil, "outless-totp-encryption", 32)
	if err != nil {
		panic(fmt.Sprintf("HKDF key derivation failed: %v", err))
	}
	copy(key[:], derived)
	return key
}

// Encrypt encrypts plaintext using AES-GCM with the provided key.
// Returns base64-encoded ciphertext (nonce prepended).
func Encrypt(key [32]byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded AES-GCM ciphertext (nonce prepended).
func Decrypt(key [32]byte, encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding base64: %w", err)
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a string looks like an encrypted base64 blob
// (at least 32 bytes after decoding, which covers nonce + at least 1 byte).
func IsEncrypted(s string) bool {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return false
	}
	return len(data) >= 28 // 12-byte nonce + 16-byte GCM tag minimum
}
