package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "outless/internal/adapters/http"
	"outless/internal/adapters/postgres"
	"outless/internal/app/subscription"

	"golang.org/x/sync/errgroup"
)

// Config defines API process settings.
type Config struct {
	DatabaseURL     string
	HTTPAddress     string
	ShutdownTimeout time.Duration
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	nodeRepo := postgres.NewGormNodeRepository(db, logger)
	tokenRepo := postgres.NewGormTokenRepository(db, logger)
	subscriptionService := subscription.NewService(nodeRepo, tokenRepo)
	handler := httpadapter.NewSubscriptionHandler(subscriptionService, logger)
	server := httpadapter.NewServer(httpadapter.Config{Address: cfg.HTTPAddress}, logger, handler)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return server.Start()
	})

	group.Go(func() error {
		<-groupCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("graceful shutdown failed: %w", shutdownErr)
		}
		return nil
	})

	if err = group.Wait(); err != nil && ctx.Err() == nil {
		logger.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func loadConfig() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://outless:outless@localhost:5432/outless?sslmode=disable"
	}

	httpAddress := os.Getenv("HTTP_ADDR")
	if httpAddress == "" {
		httpAddress = ":41220"
	}

	shutdownTimeout := 10 * time.Second

	return Config{
		DatabaseURL:     databaseURL,
		HTTPAddress:     httpAddress,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}
