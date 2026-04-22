package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"outless/internal/adapters/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	owner := flag.String("owner", "dev-user", "token owner label")
	ttl := flag.Duration("ttl", 30*24*time.Hour, "token ttl")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://outless:outless@localhost:5432/outless?sslmode=disable"
	}

	db, err := postgres.NewGormDB(databaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	repo := postgres.NewGormTokenRepository(db, logger)
	expiresAt := time.Now().UTC().Add(*ttl)
	token, err := repo.IssueToken(context.Background(), *owner, expiresAt)
	if err != nil {
		logger.Error("failed to issue token", slog.String("error", err.Error()))
		os.Exit(1)
	}

	fmt.Printf("TOKEN=%s\n", token)
	fmt.Printf("EXPIRES_AT=%s\n", expiresAt.Format(time.RFC3339))
}
