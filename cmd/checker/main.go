package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"outless/internal/adapters/postgres"
	"outless/internal/adapters/xray"
	"outless/internal/app/checker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	db, err := postgres.NewGormDB(databaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	repo := postgres.NewGormNodeRepository(db, logger)
	engine := xray.NewEngine(&http.Client{Timeout: 10 * time.Second}, logger, "https://www.google.com/generate_204", "http://xray:10085")
	service := checker.NewService(repo, engine, logger, checker.Config{Workers: 16})

	if err = service.RunOnce(ctx); err != nil {
		logger.Error("checker run failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
