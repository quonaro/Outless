package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"outless/internal/adapters/postgres"
	"outless/internal/app/hub"
	"outless/pkg/config"
)

func main() {
	configPath := flag.String("config", "outless.yaml", "path to config file")
	listenAddr := flag.String("addr", ":443", "address to listen on")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig(*configPath, logger)
	if err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	db, err := postgres.NewGormDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect postgres orm", slog.String("error", err.Error()))
		os.Exit(1)
	}

	nodeRepo := postgres.NewGormNodeRepository(db, logger)
	tokenRepo := postgres.NewGormTokenRepository(db, logger)
	hubService := hub.NewService(nodeRepo, tokenRepo, logger)

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		logger.Error("failed to listen", slog.String("addr", *listenAddr), slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("hub started", slog.String("addr", *listenAddr))

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					logger.Error("accept error", slog.String("error", err.Error()))
				}
				continue
			}

			go handleConnection(ctx, hubService, conn)
		}
	}()

	<-ctx.Done()
	logger.Info("hub shutting down")
	listener.Close()
}

func handleConnection(ctx context.Context, service *hub.Service, conn net.Conn) {
	// Extract token from connection (TODO: implement token extraction from SNI/path)
	// For now, we'll use a placeholder
	token := "placeholder"

	if err := service.HandleConnection(ctx, conn, token); err != nil {
		slog.Error("connection handler error", slog.String("remote_addr", conn.RemoteAddr().String()), slog.String("error", err.Error()))
	}
}

type Config struct {
	DatabaseURL string
}

func loadConfig(path string, logger *slog.Logger) (Config, error) {
	loader := config.NewLoader(logger)
	yamlCfg := config.DefaultConfig()

	if err := loader.LoadOrCreate(path, &yamlCfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	return Config{
		DatabaseURL: yamlCfg.Database.URL,
	}, nil
}
