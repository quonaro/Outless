package config

import (
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestGenerateRealityKeyPair(t *testing.T) {
	priv, pub, err := GenerateRealityKeyPair()
	if err != nil {
		t.Fatalf("GenerateRealityKeyPair failed: %v", err)
	}
	if priv == "" || pub == "" {
		t.Fatal("GenerateRealityKeyPair returned empty keys")
	}

	privBytes, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("failed to decode private key: %v", err)
	}
	if len(privBytes) != curve25519.ScalarSize {
		t.Fatalf("private key length %d, want %d", len(privBytes), curve25519.ScalarSize)
	}

	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("failed to decode public key: %v", err)
	}
	if len(pubBytes) != curve25519.ScalarSize {
		t.Fatalf("public key length %d, want %d", len(pubBytes), curve25519.ScalarSize)
	}

	derivedPub, err := curve25519.X25519(privBytes, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("failed to derive public key: %v", err)
	}
	if string(derivedPub) != string(pubBytes) {
		t.Fatal("derived public key does not match generated public key")
	}
}

func TestDeriveRealityPublicKey(t *testing.T) {
	priv, pub, err := GenerateRealityKeyPair()
	if err != nil {
		t.Fatalf("GenerateRealityKeyPair failed: %v", err)
	}

	derivedPub, err := DeriveRealityPublicKey(priv)
	if err != nil {
		t.Fatalf("DeriveRealityPublicKey failed: %v", err)
	}
	if derivedPub != pub {
		t.Fatal("derived public key does not match generated public key")
	}
}
