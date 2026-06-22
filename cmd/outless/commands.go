package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"outless/shared/config"
	"outless/shared/logging"

	"github.com/quonaro/lota/engine"
	"gopkg.in/yaml.v3"
)

func showConfig(_ context.Context, nctx engine.NativeContext) error {
	cfgPath := nctx.Vars["CONFIG_PATH"]
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	logger := logging.New("outless")
	loader := config.NewLoader(logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(cfgPath, &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	withSecrets := nctx.Args["with-secrets"]
	if withSecrets != "true" {
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err == nil {
			if jwt, ok := raw["jwt"].(map[string]any); ok {
				jwt["secret"] = "***"
			}
			if out, err := yaml.Marshal(raw); err == nil {
				data = out
			}
		}
	}

	_, _ = nctx.Stdout.Write(data)
	return nil
}

func validateConfig(_ context.Context, nctx engine.NativeContext) error {
	cfgPath := nctx.Vars["CONFIG_PATH"]
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	logger := logging.New("outless")
	loader := config.NewLoader(logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(cfgPath, &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	_, _ = fmt.Fprintln(nctx.Stdout, "Configuration is valid")
	return nil
}

func backupDB(_ context.Context, nctx engine.NativeContext) error {
	cfgPath := nctx.Vars["CONFIG_PATH"]
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	logger := logging.New("outless")
	loader := config.NewLoader(logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(cfgPath, &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dbPath := string(cfg.Database)
	if dbPath == "" {
		return fmt.Errorf("database path is empty")
	}

	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("database file not found: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	backupDir := filepath.Dir(dbPath)
	backupName := fmt.Sprintf("outless_%s.db.tar.gz", timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	outFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("creating backup file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	gw := gzip.NewWriter(outFile)
	defer func() { _ = gw.Close() }()

	tw := tar.NewWriter(gw)
	defer func() { _ = tw.Close() }()

	dbFile, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = dbFile.Close() }()

	info, err := dbFile.Stat()
	if err != nil {
		return fmt.Errorf("stating database: %w", err)
	}

	hdr := &tar.Header{
		Name: filepath.Base(dbPath),
		Mode: 0o600,
		Size: info.Size(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("writing tar header: %w", err)
	}
	if _, err := io.Copy(tw, dbFile); err != nil {
		return fmt.Errorf("writing database to tar: %w", err)
	}

	_, _ = fmt.Fprintf(nctx.Stdout, "Backup created: %s\n", backupPath)
	return nil
}

func checkStatus(_ context.Context, nctx engine.NativeContext) error {
	cfgPath := nctx.Vars["CONFIG_PATH"]
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	logger := logging.New("outless")
	loader := config.NewLoader(logger)
	cfg := config.DefaultConfig()
	if err := loader.LoadOrCreate(cfgPath, &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	addr := fmt.Sprintf("localhost:%d", cfg.App.HTTPPort)
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		_, _ = fmt.Fprintf(nctx.Stdout, "Server is not running (port %d is unreachable)\n", cfg.App.HTTPPort)
		return nil
	}
	_ = conn.Close()

	_, _ = fmt.Fprintf(nctx.Stdout, "Server is running on port %d\n", cfg.App.HTTPPort)
	return nil
}
