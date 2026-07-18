package country

import (
	"context"

	"outless/internal/domain"
)

// Provider looks up country information for an IP address.
type Provider interface {
	Name() string
	Lookup(ctx context.Context, ip string) (domain.CountryInfo, error)
}
