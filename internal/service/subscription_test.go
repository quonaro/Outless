package service

import (
	"testing"

	"outless/internal/domain"
)

const (
	testOriginGroup = "origin"
	testHubGroup    = "hub"
)

func TestNodeUsesOriginsAndHub(t *testing.T) {
	s := &SubscriptionService{}
	groups := map[string]domain.Group{
		testOriginGroup: {ID: testOriginGroup, ShowOrigins: true},
		testHubGroup:    {ID: testHubGroup, ShowOrigins: false},
	}

	tests := []struct {
		name        string
		nodeGroups  []string
		tokenGroups []string
		wantOrigin  bool
		wantHub     bool
	}{
		{"origin only", []string{testOriginGroup}, []string{testOriginGroup}, true, false},
		{"hub only", []string{testHubGroup}, []string{testHubGroup}, false, true},
		{"mixed groups", []string{testOriginGroup, testHubGroup}, []string{testOriginGroup, testHubGroup}, true, true},
		{"node in both but token only origin", []string{testOriginGroup, testHubGroup}, []string{testOriginGroup}, true, false},
		{"node in both but token only hub", []string{testOriginGroup, testHubGroup}, []string{testHubGroup}, false, true},
		{"all groups allowed", []string{testOriginGroup, testHubGroup}, nil, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := domain.Node{ID: "n", GroupIDs: tt.nodeGroups}
			token := domain.Token{GroupIDs: tt.tokenGroups}
			if got := s.nodeUsesOrigins(node, groups, token); got != tt.wantOrigin {
				t.Errorf("nodeUsesOrigins() = %v, want %v", got, tt.wantOrigin)
			}
			if got := s.nodeUsesHub(node, groups, token); got != tt.wantHub {
				t.Errorf("nodeUsesHub() = %v, want %v", got, tt.wantHub)
			}
		})
	}
}
