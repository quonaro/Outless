// Package ssrf provides URL validation to prevent Server-Side Request Forgery.
package ssrf

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// ValidateURL checks that a URL is safe to fetch from the server.
// It rejects non-HTTP(S) schemes, private/loopback/link-local IPs,
// and common internal hostnames.
func ValidateURL(rawURL string) error {
	return ValidateURLWithContext(context.Background(), rawURL)
}

// ValidateURLWithContext is like ValidateURL but accepts a context for DNS resolution.
func ValidateURLWithContext(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q is not allowed", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("host is empty")
	}

	if isBlockedHost(host) {
		return fmt.Errorf("host %q is blocked", host)
	}

	resolver := net.DefaultResolver
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolving host %q: %w", host, err)
	}

	for _, addr := range addrs {
		if isBlockedIP(addr.IP) {
			return fmt.Errorf("IP %s of host %q is blocked", addr.IP, host)
		}
	}

	return nil
}

// CheckRedirect validates each redirect target during HTTP requests.
func CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("too many redirects")
	}
	return ValidateURL(req.URL.String())
}

// isBlockedHost checks for common internal hostnames.
func isBlockedHost(host string) bool {
	lower := strings.ToLower(host)
	switch lower {
	case "localhost", "localhost.localdomain":
		return true
	}
	return false
}

// isBlockedIP returns true for private, loopback, unspecified, and link-local IPs.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ip.IsLinkLocalMulticast() {
		return true
	}
	return false
}
