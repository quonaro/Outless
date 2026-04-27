package geoip

import (
	"context"

	"outless/internal/domain"
)

// NullGeoIPResolver implements GeoIPResolver with no-op behavior.
// Used when GeoIP is not configured.
type NullGeoIPResolver struct{}

// NewNullGeoIPResolver creates a new no-op GeoIP resolver.
func NewNullGeoIPResolver() *NullGeoIPResolver {
	return &NullGeoIPResolver{}
}

// LookupCountry always returns empty string.
func (n *NullGeoIPResolver) LookupCountry(ctx context.Context, ip string) (string, error) {
	return "", nil
}

// Close is a no-op.
func (n *NullGeoIPResolver) Close() error {
	return nil
}

var _ domain.GeoIPResolver = (*NullGeoIPResolver)(nil)
