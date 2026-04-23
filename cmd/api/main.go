package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "outless/internal/adapters/http"
	"outless/internal/adapters/postgres"
	"outless/internal/app/auth"
	"outless/internal/app/public"
	"outless/internal/app/subscription"
	"outless/internal/domain"
	"outless/pkg/config"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
)

// Config defines API process settings.
type Config struct {
	DatabaseURL     string
	HTTPAddress     string
	JWTSecret       string
	JWTExpiry       time.Duration
	ShutdownTimeout time.Duration
	OutlessLogin    string
	OutlessPassword string
}

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := loadConfig(*configPath, logger)
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
	groupRepo := postgres.NewGormGroupRepository(db, logger)
	publicSourceRepo := postgres.NewGormPublicSourceRepository(db, logger)
	adminRepo := postgres.NewGormAdminRepository(db, logger)
	if err = bootstrapAdminFromEnv(ctx, adminRepo, cfg, logger); err != nil {
		logger.Error("failed to bootstrap admin from env", slog.String("error", err.Error()))
		os.Exit(1)
	}

	jwtService := auth.NewJWTService(cfg.JWTSecret, cfg.JWTExpiry)
	subscriptionService := subscription.NewService(nodeRepo, tokenRepo)
	publicService := public.NewService(nodeRepo, publicSourceRepo, groupRepo, logger)
	authHandler := httpadapter.NewAuthHandler(adminRepo, jwtService, logger)
	subscriptionHandler := httpadapter.NewSubscriptionHandler(subscriptionService, logger)
	tokenHandler := httpadapter.NewTokenManagementHandler(tokenRepo, groupRepo, logger)
	nodeHandler := httpadapter.NewNodeManagementHandler(nodeRepo, groupRepo, logger)
	groupHandler := httpadapter.NewGroupManagementHandler(groupRepo, logger)
	publicSourceHandler := httpadapter.NewPublicSourceManagementHandler(publicSourceRepo, groupRepo, publicService, logger)
	settingsHandler := httpadapter.NewSettingsHandler(*configPath, logger)
	adminHandler := httpadapter.NewAdminManagementHandler(adminRepo, logger)
	server := httpadapter.NewServer(httpadapter.Config{Address: cfg.HTTPAddress}, logger, subscriptionHandler, authHandler, tokenHandler, nodeHandler, groupHandler, publicSourceHandler, settingsHandler, adminHandler)

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

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	return Config{
		DatabaseURL:     yamlCfg.Database.URL,
		HTTPAddress:     ":41220",
		JWTSecret:       yamlCfg.API.JWT.Secret,
		JWTExpiry:       yamlCfg.API.JWT.Expiry,
		ShutdownTimeout: yamlCfg.API.ShutdownTimeout,
		OutlessLogin:    yamlCfg.API.Admin.Login,
		OutlessPassword: yamlCfg.API.Admin.Password,
	}, nil
}

func bootstrapAdminFromEnv(ctx context.Context, adminRepo domain.AdminRepository, cfg Config, logger *slog.Logger) error {
	if cfg.OutlessLogin == "" || cfg.OutlessPassword == "" {
		logger.Info("admin env bootstrap disabled")
		return nil
	}

	count, err := adminRepo.Count(ctx)
	if err != nil {
		return fmt.Errorf("counting admins before env bootstrap: %w", err)
	}

	if count > 0 {
		logger.Info("admin env bootstrap skipped because admins already exist")
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.OutlessPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing OUTLESS_PASSWORD: %w", err)
	}

	admin := domain.Admin{
		ID:           newAdminID(),
		Username:     cfg.OutlessLogin,
		PasswordHash: string(passwordHash),
	}

	if err := adminRepo.Create(ctx, admin); err != nil {
		if errors.Is(err, domain.ErrAdminAlreadyExists) {
			logger.Info("admin env bootstrap skipped due race: first admin already exists")
			return nil
		}
		return fmt.Errorf("creating admin from env: %w", err)
	}

	logger.Info("admin env bootstrap completed", slog.String("username", cfg.OutlessLogin))
	return nil
}

func newAdminID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("admin_%d", time.Now().UTC().UnixNano())
	}

	return hex.EncodeToString(bytes)
}
