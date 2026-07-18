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

type ipapiisProvider struct {
	client *http.Client
}

func newIPAPIIS(client *http.Client) Provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ipapiisProvider{client: client}
}

func (p *ipapiisProvider) Name() string { return "ipapi.is" }

type ipapiisResponse struct {
	Location struct {
		CountryCode string `json:"country_code"`
		CountryName string `json:"country"`
	} `json:"location"`
}

func (p *ipapiisProvider) Lookup(ctx context.Context, ip string) (domain.CountryInfo, error) {
	u := "https://api.ipapi.is/"
	if ip != "" {
		u = fmt.Sprintf("https://api.ipapi.is/?q=%s", ip)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipapi.is: building request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipapi.is: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.CountryInfo{}, fmt.Errorf("ipapi.is: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipapi.is: reading body: %w", err)
	}

	var r ipapiisResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return domain.CountryInfo{}, fmt.Errorf("ipapi.is: decoding response: %w", err)
	}

	return domain.CountryInfo{
		CountryCode: domain.NormalizeCountryCode(r.Location.CountryCode),
		CountryName: r.Location.CountryName,
	}, nil
}
