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

type ipinfoProvider struct {
	client *http.Client
}

func newIPInfo(client *http.Client) Provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ipinfoProvider{client: client}
}

func (p *ipinfoProvider) Name() string { return "ipinfo.io" }

type ipinfoResponse struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	City    string `json:"city"`
	Region  string `json:"region"`
}

func (p *ipinfoProvider) Lookup(ctx context.Context, ip string) (domain.CountryInfo, error) {
	u := "https://ipinfo.io/json"
	if ip != "" {
		u = fmt.Sprintf("https://ipinfo.io/%s/json", ip)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipinfo.io: building request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipinfo.io: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.CountryInfo{}, fmt.Errorf("ipinfo.io: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipinfo.io: reading body: %w", err)
	}

	var r ipinfoResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipinfo.io: decoding response: %w", err)
	}

	return domain.CountryInfo{
		CountryCode: domain.NormalizeCountryCode(r.Country),
		CountryName: r.Region,
	}, nil
}
