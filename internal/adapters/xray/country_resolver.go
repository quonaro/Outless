package xray

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"

	"outless/internal/domain"
)

type countryResolver interface {
	CountryByHost(ctx context.Context, host string) (string, bool)
}

type geoIPCountryResolver struct {
	logger *slog.Logger
	db     *geoip2.Reader
	cfg    GeoIPConfig
	client *http.Client

	mu    sync.RWMutex
	cache map[string]string
	meta  geoIPMeta
}

type geoIPMeta struct {
	ETag         string
	LastModified string
	LastChecked  time.Time
}

func newGeoIPCountryResolver(logger *slog.Logger, cfg GeoIPConfig) countryResolver {
	path := strings.TrimSpace(cfg.DBPath)
	if path == "" {
		path = defaultGeoIPDBPathNearBinary(logger)
		cfg.DBPath = path
		logger.Info("geoip db path not set, using default near binary", slog.String("path", path))
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}

	r := &geoIPCountryResolver{
		logger: logger,
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		cache:  make(map[string]string, 256),
	}
	if cfg.Auto {
		if err := r.ensureFreshDB(context.Background()); err != nil {
			logger.Warn("geoip auto update failed", slog.String("error", err.Error()))
		}
	}
	if err := r.openDB(); err != nil {
		logger.Warn("geoip country resolver disabled: open db failed", slog.String("path", path), slog.String("error", err.Error()))
		return nil
	}
	logger.Info("geoip country resolver enabled",
		slog.String("path", path),
		slog.Bool("auto", cfg.Auto),
		slog.String("url", strings.TrimSpace(cfg.DBURL)),
		slog.Duration("ttl", cfg.TTL),
	)
	return r
}

func defaultGeoIPDBPathNearBinary(logger *slog.Logger) string {
	exe, err := os.Executable()
	if err != nil {
		logger.Warn("geoip default path fallback to cwd due executable path error", slog.String("error", err.Error()))
		return "GeoLite2-Country.mmdb"
	}
	return filepath.Join(filepath.Dir(exe), "GeoLite2-Country.mmdb")
}

func (r *geoIPCountryResolver) CountryByHost(ctx context.Context, host string) (string, bool) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", false
	}
	if r.cfg.Auto {
		if err := r.ensureFreshDB(ctx); err != nil {
			r.logger.Debug("geoip ensure fresh failed", slog.String("error", err.Error()))
		}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", false
	}
	key := ip.String()
	r.mu.RLock()
	if country, ok := r.cache[key]; ok {
		r.mu.RUnlock()
		return country, true
	}
	r.mu.RUnlock()

	rec, err := r.db.Country(ip)
	if err != nil {
		r.logger.Debug("geoip lookup failed", slog.String("ip", key), slog.String("error", err.Error()))
		return "", false
	}
	country := domain.NormalizeCountryCode(rec.Country.IsoCode)
	if len(country) != 2 {
		return "", false
	}

	r.mu.Lock()
	r.cache[key] = country
	r.mu.Unlock()
	return country, true
}

func (r *geoIPCountryResolver) openDB() error {
	db, err := geoip2.Open(r.cfg.DBPath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	old := r.db
	r.db = db
	r.cache = make(map[string]string, 256)
	r.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (r *geoIPCountryResolver) ensureFreshDB(ctx context.Context) error {
	now := time.Now()
	r.mu.RLock()
	last := r.meta.LastChecked
	ttl := r.cfg.TTL
	r.mu.RUnlock()
	if !last.IsZero() && now.Sub(last) < ttl {
		return nil
	}

	r.mu.Lock()
	if !r.meta.LastChecked.IsZero() && time.Since(r.meta.LastChecked) < ttl {
		r.mu.Unlock()
		return nil
	}
	r.meta.LastChecked = now
	etag := r.meta.ETag
	lm := r.meta.LastModified
	r.mu.Unlock()

	url := strings.TrimSpace(r.cfg.DBURL)
	if url == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lm != "" {
		req.Header.Set("If-Modified-Since", lm)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geoip download status %d", resp.StatusCode)
	}

	if err = os.MkdirAll(filepath.Dir(r.cfg.DBPath), 0o755); err != nil {
		return err
	}
	tmp := r.cfg.DBPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, io.LimitReader(resp.Body, 128<<20)); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if want, err := strconv.ParseInt(cl, 10, 64); err == nil {
			if st, err := os.Stat(tmp); err == nil && st.Size() != want {
				_ = os.Remove(tmp)
				return fmt.Errorf("geoip download size mismatch: got %d want %d", st.Size(), want)
			}
		}
	}
	if err = os.Rename(tmp, r.cfg.DBPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err = r.openDB(); err != nil {
		return err
	}

	r.mu.Lock()
	if v := strings.TrimSpace(resp.Header.Get("ETag")); v != "" {
		r.meta.ETag = v
	}
	if v := strings.TrimSpace(resp.Header.Get("Last-Modified")); v != "" {
		r.meta.LastModified = v
	}
	r.mu.Unlock()

	r.logger.Info("geoip db updated",
		slog.String("path", r.cfg.DBPath),
		slog.String("url", url),
		slog.String("etag", r.meta.ETag),
		slog.String("last_modified", r.meta.LastModified),
	)
	return nil
}
