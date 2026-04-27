package geoip

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"

	"outless/internal/domain"

	"github.com/oschwald/geoip2-golang"
)

// MaxMindAdapter implements GeoIPResolver using MaxMind GeoIP2 database.
type MaxMindAdapter struct {
	db     *geoip2.Reader
	logger *slog.Logger
}

// NewMaxMindAdapter creates a new MaxMind GeoIP2 resolver.
func NewMaxMindAdapter(dbPath string, logger *slog.Logger) (*MaxMindAdapter, error) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening geoip database: %w", err)
	}

	logger.Info("geoip database loaded", slog.String("path", dbPath))

	return &MaxMindAdapter{
		db:     db,
		logger: logger,
	}, nil
}

// LookupCountry returns the ISO 3166 alpha-2 country code for the given IP address.
func (a *MaxMindAdapter) LookupCountry(ctx context.Context, ip string) (string, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "", fmt.Errorf("parsing IP address %s: %w", ip, err)
	}

	country, err := a.db.Country(addr.AsSlice())
	if err != nil {
		return "", fmt.Errorf("looking up country for %s: %w", ip, err)
	}

	// MaxMind returns ISO 3166-1 alpha-2 codes (e.g., "US", "DE")
	countryCode := country.Country.IsoCode
	if countryCode == "" {
		return "", nil // Unknown country
	}

	return domain.NormalizeCountryCode(countryCode), nil
}

// Close releases the GeoIP database resources.
func (a *MaxMindAdapter) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}
