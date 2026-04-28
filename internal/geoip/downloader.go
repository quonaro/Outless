package geoip

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	GeoIPDBFileName        = "GeoLite2-Country.mmdb"
	GeoIPDownloadURL       = "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"
	defaultDownloadTimeout = 30 * time.Second
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)

// DownloadGeoIP downloads the GeoIP database if it doesn't exist.
// dbPath is the full path to the GeoIP database file.
func DownloadGeoIP(dbPath string, logger *slog.Logger) error {
	// If dbPath is a directory, append the default filename
	info, err := os.Stat(dbPath)
	if err == nil && info.IsDir() {
		dbPath = filepath.Join(dbPath, GeoIPDBFileName)
	} else if err == nil && !info.IsDir() {
		// dbPath is already a file path, use it as-is
	} else if os.IsNotExist(err) {
		// Path doesn't exist - check if it looks like a directory or file
		if filepath.Ext(dbPath) == "" {
			// No extension, treat as directory
			dbPath = filepath.Join(dbPath, GeoIPDBFileName)
		}
		// Otherwise treat as file path
	} else {
		return fmt.Errorf("failed to check GeoIP database path: %w", err)
	}

	// Check if file already exists and has content
	info, err = os.Stat(dbPath)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("GeoIP database path is a directory, not a file: %s", dbPath)
		}
		if info.Size() > 0 {
			logger.Info("geoip database already exists", slog.String("path", dbPath), slog.Int64("size", info.Size()))
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check GeoIP database file: %w", err)
	}

	logger.Info("geoip database not found, downloading", slog.String("url", GeoIPDownloadURL))

	client := &http.Client{
		Timeout: defaultDownloadTimeout,
	}

	resp, err := client.Get(GeoIPDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download GeoIP database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download GeoIP database: HTTP %d", resp.StatusCode)
	}

	// Create the file
	outFile, err := os.Create(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create GeoIP database file: %w", err)
	}
	defer outFile.Close()

	// Copy the response body to the file
	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write GeoIP database: %w", err)
	}

	logger.Info("geoip database downloaded successfully", slog.String("path", dbPath))
	return nil
}
