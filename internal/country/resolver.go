package country

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"outless/internal/domain"
)

// DefaultCooldown is the backoff applied after a rate-limit.
const DefaultCooldown = 10 * time.Minute

// FailureCooldown is the backoff applied after any other provider failure.
const FailureCooldown = 1 * time.Minute

// CacheTTL is how long successful lookups are kept in memory.
const CacheTTL = 1 * time.Hour

// Resolver queries built-in IP geolocation providers in rotation.
type Resolver struct {
	client      *http.Client
	providers   []Provider
	cooldown    time.Duration
	mu          sync.RWMutex
	cache       map[string]cacheEntry
	providerTTL map[string]time.Time
}

type cacheEntry struct {
	info      domain.CountryInfo
	expiresAt time.Time
}

// NewResolver creates a resolver with the built-in provider set.
func NewResolver(client *http.Client) *Resolver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Resolver{
		client:      client,
		providers:   defaultProviders(client),
		cooldown:    DefaultCooldown,
		cache:       make(map[string]cacheEntry),
		providerTTL: make(map[string]time.Time),
	}
}

func defaultProviders(client *http.Client) []Provider {
	return []Provider{
		newIPWhois(client),
		newIPAPI(client),
		newIPInfo(client),
		newIPAPIIS(client),
	}
}

// Lookup returns country information for the given IP. An empty ip triggers a
// self-IP lookup using the caller's public address.
func (r *Resolver) Lookup(ctx context.Context, ip string) (domain.CountryInfo, error) {
	key := ip
	if key == "" {
		key = "self"
	}

	r.mu.RLock()
	if entry, ok := r.cache[key]; ok && entry.expiresAt.After(time.Now().UTC()) {
		r.mu.RUnlock()
		return entry.info, nil
	}
	r.mu.RUnlock()

	err := r.rotateProviders(ctx, ip, func(p Provider) (domain.CountryInfo, error) {
		return p.Lookup(ctx, ip)
	})
	if err != nil {
		return domain.CountryInfo{}, err
	}

	// Should be filled by rotateProviders; read safely.
	r.mu.RLock()
	info, ok := r.cache[key]
	r.mu.RUnlock()
	if !ok {
		return domain.CountryInfo{}, fmt.Errorf("all providers failed")
	}
	return info.info, nil
}

// ResolveHost converts a host (IP or domain) into an IPv4/IPv6 address.
func (r *Resolver) ResolveHost(ctx context.Context, host string) (string, error) {
	if host == "" {
		return "", errors.New("empty host")
	}
	if net.ParseIP(host) != nil {
		return host, nil
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", fmt.Errorf("resolving host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IPs found for host %q", host)
	}
	return ips[0].IP.String(), nil
}

func (r *Resolver) rotateProviders(
	ctx context.Context,
	key string,
	fn func(Provider) (domain.CountryInfo, error),
) error {
	r.mu.RLock()
	now := time.Now().UTC()
	available := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		ttl, ok := r.providerTTL[p.Name()]
		if !ok || ttl.Before(now) {
			available = append(available, p)
		}
	}
	r.mu.RUnlock()

	if len(available) == 0 {
		return fmt.Errorf("all providers are rate-limited")
	}

	var lastErr error
	for _, p := range available {
		info, err := fn(p)
		if err == nil {
			info.Flag = FlagEmoji(info.CountryCode)
			if info.Flag == "" {
				info.Flag = "🏳️"
			}
			r.storeCache(key, info)
			return nil
		}
		lastErr = err
		if isRateLimit(err) {
			r.markProviderCooldown(p.Name(), r.cooldown)
		} else {
			r.markProviderCooldown(p.Name(), FailureCooldown)
		}
	}

	return fmt.Errorf("all providers failed: %w", lastErr)
}

func (r *Resolver) storeCache(key string, info domain.CountryInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[key] = cacheEntry{info: info, expiresAt: time.Now().UTC().Add(CacheTTL)}
}

func (r *Resolver) markProviderCooldown(name string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providerTTL[name] = time.Now().UTC().Add(duration)
}

func isRateLimit(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "rate") ||
		strings.Contains(s, "429") ||
		strings.Contains(s, "too many")
}
