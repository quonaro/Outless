package service

import (
	"math/rand"
	"strconv"
	"strings"

	"outless/internal/domain"
)

// TrafficSnapshotProvider provides real-time traffic snapshot data.
// Implemented by domain.RuntimeController.
type TrafficSnapshotProvider interface {
	TrafficSnapshot() *domain.TrafficSnapshot
}

// pickHubForNode selects a single hub for a given node using weighted random
// based on active connection counts. Less loaded inbounds have higher weight.
// If no runtime is available or snapshot is nil, picks uniformly at random.
func (s *SubscriptionService) pickHubForNode(hubs []HubConfig) HubConfig {
	if len(hubs) == 1 {
		return hubs[0]
	}

	counts := s.inboundConnectionCounts(hubs)

	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	weights := make([]int, len(hubs))
	totalWeight := 0
	for i, c := range counts {
		w := maxCount - c + 1
		weights[i] = w
		totalWeight += w
	}

	r := rand.Intn(totalWeight)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return hubs[i]
		}
	}

	return hubs[len(hubs)-1]
}

// inboundConnectionCounts returns active connection counts aligned with hubs.
// Uses the tagIndex field on HubConfig to match sing-box inbound tags.
func (s *SubscriptionService) inboundConnectionCounts(hubs []HubConfig) []int {
	counts := make([]int, len(hubs))
	if s.runtime == nil {
		return counts
	}

	snapshot := s.runtime.TrafficSnapshot()
	if snapshot == nil {
		return counts
	}

	for _, conn := range snapshot.Connections {
		idx := parseInboundTagIndex(conn.Inbound)
		if idx < 0 {
			continue
		}
		for i, hub := range hubs {
			if hub.tagIndex == idx {
				counts[i]++
				break
			}
		}
	}

	return counts
}

// parseInboundTagIndex extracts the numeric suffix from a sing-box inbound tag
// like "vless-in-0" and returns it. Returns -1 if the tag doesn't match.
func parseInboundTagIndex(tag string) int {
	const prefix = "vless-in-"
	if !strings.HasPrefix(tag, prefix) {
		return -1
	}
	n, err := strconv.Atoi(tag[len(prefix):])
	if err != nil {
		return -1
	}
	return n
}
