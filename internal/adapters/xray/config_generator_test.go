package xray

import (
	"strings"
	"testing"
	"time"

	"outless/internal/domain"
)

func TestBuildClients(t *testing.T) {
	tokens := []domain.Token{
		{
			ID:        "token-1",
			Owner:     "user1",
			UUID:      "uuid-1",
			IsActive:  true,
			GroupIDs:  []string{"group-1"},
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		},
	}

	nodes := []domain.Node{
		{
			ID:      "node-1",
			URL:     "vless://uuid1@example.com:443",
			GroupID: "group-1",
			Status:  domain.NodeStatusHealthy,
			Country: "US",
		},
		{
			ID:      "node-2",
			URL:     "vless://uuid2@example.com:443",
			GroupID: "group-1",
			Status:  domain.NodeStatusHealthy,
			Country: "DE",
		},
	}

	clients := buildClients(tokens, nodes)

	// Should have 2 client entries (token-1 + node-1, token-1 + node-2)
	if len(clients) != 2 {
		t.Errorf("Expected 2 clients, got %d", len(clients))
	}

	// Check that each client has a node-specific email
	for _, client := range clients {
		email, ok := client["email"].(string)
		if !ok {
			t.Errorf("Client email is not a string")
			continue
		}
		if !strings.Contains(email, "-node-") {
			t.Errorf("Email doesn't contain node identifier: %s", email)
		}
	}
}

func TestBuildDirectRouting(t *testing.T) {
	clients := []map[string]any{
		{
			"id":    "uuid-1",
			"email": "token-1-node-1@outless",
			"flow":  "xtls-rprx-vision",
			"level": 0,
		},
		{
			"id":    "uuid-1",
			"email": "token-1-node-2@outless",
			"flow":  "xtls-rprx-vision",
			"level": 0,
		},
	}

	rules := buildDirectRouting(clients)

	// Should have 2 routing rules
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}

	// Check that each rule has the correct outbound tag
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			t.Errorf("Rule is not a map")
			continue
		}
		if ruleMap["type"] != "field" {
			t.Errorf("Rule type is not 'field'")
		}
		if ruleMap["outboundTag"] == nil {
			t.Errorf("Rule missing outboundTag")
		}
	}
}
