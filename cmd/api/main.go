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
	"outless/internal/app/auth"
	"outless/internal/app/subscription"

	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

// Config defines API process settings.
type Config struct {
	DatabaseURL     string
	HTTPAddress     string
	JWTSecret       string
	JWTExpiry       time.Duration
	ShutdownTimeout time.Duration
}

func main() {
	// Load .env file if present (ignore error if file doesn't exist)
	_ = godotenv.Load()

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
	adminRepo := postgres.NewGormAdminRepository(db, logger)
	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiry)
	subscriptionService := subscription.NewService(nodeRepo, tokenRepo)
	authHandler := httpadapter.NewAuthHandler(adminRepo, jwtService, logger)
	subscriptionHandler := httpadapter.NewSubscriptionHandler(subscriptionService, logger)
	server := httpadapter.NewServer(httpadapter.Config{Address: cfg.HTTPAddress}, logger, subscriptionHandler, authHandler)

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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	jwtExpiry := 24 * time.Hour

	shutdownTimeout := 10 * time.Second

	return Config{
		DatabaseURL:     databaseURL,
		HTTPAddress:     httpAddress,
		JWTSecret:       jwtSecret,
		JWTExpiry:       jwtExpiry,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}
