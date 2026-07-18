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

type ipapiProvider struct {
	client *http.Client
}

func newIPAPI(client *http.Client) Provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ipapiProvider{client: client}
}

func (p *ipapiProvider) Name() string { return "ip-api.com" }

type ipapiResponse struct {
	Status      string `json:"status"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"country"`
}

func (p *ipapiProvider) Lookup(ctx context.Context, ip string) (domain.CountryInfo, error) {
	u := "http://ip-api.com/json/"
	if ip != "" {
		u = fmt.Sprintf("http://ip-api.com/json/%s", ip)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ip-api.com: building request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ip-api.com: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.CountryInfo{}, fmt.Errorf("ip-api.com: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ip-api.com: reading body: %w", err)
	}

	var r ipapiResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ip-api.com: decoding response: %w", err)
	}
	if r.Status != "success" {
		return domain.CountryInfo{}, fmt.Errorf("ip-api.com: status %s", r.Status)
	}

	return domain.CountryInfo{
		CountryCode: domain.NormalizeCountryCode(r.CountryCode),
		CountryName: r.CountryName,
	}, nil
}
