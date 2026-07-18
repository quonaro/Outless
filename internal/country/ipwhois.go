package country

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"outless/internal/domain"
)

type ipwhoisProvider struct {
	client *http.Client
}

func newIPWhois(client *http.Client) Provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ipwhoisProvider{client: client}
}

func (p *ipwhoisProvider) Name() string { return "ipwho.is" }

type ipwhoisResponse struct {
	Success     bool   `json:"success"`
	CountryCode string `json:"country_code"`
	CountryName string `json:"country"`
}

func (p *ipwhoisProvider) Lookup(ctx context.Context, ip string) (domain.CountryInfo, error) {
	u := "https://ipwho.is/"
	if ip != "" {
		u = fmt.Sprintf("https://ipwho.is/%s", ip)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipwho.is: building request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipwho.is: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.CountryInfo{}, fmt.Errorf("ipwho.is: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipwho.is: reading body: %w", err)
	}

	var r ipwhoisResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipwho.is: decoding response: %w", err)
	}
	if !r.Success {
		return domain.CountryInfo{}, fmt.Errorf("ipwho.is: lookup failed")
	}

	return domain.CountryInfo{
		CountryCode: domain.NormalizeCountryCode(r.CountryCode),
		CountryName: r.CountryName,
	}, nil
}
