package crypto

import (
	"testing"
)

func TestHashTokenNode(t *testing.T) {
	tests := []struct {
		name     string
		tokenID  string
		nodeID   string
		want     string
		wantEmpty bool
	}{
		{
			name:    "valid inputs",
			tokenID: "token-123",
			nodeID:  "node-456",
			wantEmpty: false,
		},
		{
			name:     "empty token ID",
			tokenID:  "",
			nodeID:   "node-456",
			wantEmpty: true,
		},
		{
			name:     "empty node ID",
			tokenID:  "token-123",
			nodeID:   "",
			wantEmpty: true,
		},
		{
			name:     "both empty",
			tokenID:  "",
			nodeID:   "",
			wantEmpty: true,
		},
		{
			name:    "same inputs produce same hash",
			tokenID: "token-123",
			nodeID:  "node-456",
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashTokenNode(tt.tokenID, tt.nodeID)
			
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("HashTokenNode() = %v, want empty", got)
				}
				return
			}
			
			if got == "" {
				t.Errorf("HashTokenNode() returned empty, want non-empty")
			}
			
			// MD5 hash should be 32 hex characters
			if len(got) != 32 {
				t.Errorf("HashTokenNode() length = %v, want 32", len(got))
			}
			
			// Test determinism
			if tt.name == "same inputs produce same hash" {
				got2 := HashTokenNode(tt.tokenID, tt.nodeID)
				if got != got2 {
					t.Errorf("HashTokenNode() not deterministic: %v != %v", got, got2)
				}
			}
			
			// Test that different inputs produce different hashes
			if tt.name == "valid inputs" {
				got2 := HashTokenNode("different-token", tt.nodeID)
				if got == got2 {
					t.Errorf("HashTokenNode() produces same hash for different token IDs")
				}
				got3 := HashTokenNode(tt.tokenID, "different-node")
				if got == got3 {
					t.Errorf("HashTokenNode() produces same hash for different node IDs")
				}
			}
		})
	}
}

func TestHashEmail(t *testing.T) {
	tests := []struct {
		name string
		hash string
		want string
	}{
		{
			name: "valid hash",
			hash: "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
			want: "h-a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6@outless",
		},
		{
			name: "empty hash",
			hash: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashEmail(tt.hash)
			if got != tt.want {
				t.Errorf("HashEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}
