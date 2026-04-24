package postgres

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrateDatabase applies pending migrations to the database.
// It uses the migrations directory relative to the current working directory.
func MigrateDatabase(dbURL string, migrationsDir string, logger *slog.Logger) error {
	migrationsPath := "file://" + migrationsDir

	// Check if migrations directory exists
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		logger.Error("migrations directory does not exist", slog.String("path", migrationsDir))
		return fmt.Errorf("migrations directory does not exist: %s", migrationsDir)
	}

	// List files in migrations directory for debugging
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		logger.Error("failed to read migrations directory", slog.String("path", migrationsDir), slog.String("error", err.Error()))
	} else {
		logger.Info("migrations directory contents", slog.String("path", migrationsDir), slog.Int("file_count", len(entries)))
		for _, entry := range entries {
			logger.Info("migration file", slog.String("name", entry.Name()))
		}
	}

	logger.Info("running migrations", slog.String("path", migrationsPath), slog.String("db_url", dbURL))

	m, err := migrate.New(
		migrationsPath,
		dbURL,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MIGRATE NEW ERROR: %v (type: %T)\n", err, err)
		logger.Error("failed to create migrate instance", slog.String("error", err.Error()), slog.String("error_type", fmt.Sprintf("%T", err)))
		return fmt.Errorf("creating migrate instance: %w", err)
	}

	defer m.Close()

	// Set a timeout for migration operations
	m.Log = &migrateLogger{logger: logger}

	logger.Info("applying migrations")
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		fmt.Fprintf(os.Stderr, "MIGRATION ERROR: %v (type: %T)\n", err, err)
		logger.Error("failed to apply migrations", slog.String("error", err.Error()), slog.String("error_type", fmt.Sprintf("%T", err)))
		return fmt.Errorf("running migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		logger.Info("database is up to date, no migrations needed")
		return nil
	}

	version, dirty, err := m.Version()
	if err != nil {
		logger.Error("failed to get migration version", slog.String("error", err.Error()))
		return fmt.Errorf("getting migration version: %w", err)
	}

	logger.Info("migrations applied successfully", slog.Uint64("version", uint64(version)), slog.Bool("dirty", dirty))
	return nil
}

// migrateLogger implements migrate.Logger for structured logging.
type migrateLogger struct {
	logger *slog.Logger
}

func (l *migrateLogger) Printf(format string, v ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, v...))
}

func (l *migrateLogger) Verbose() bool {
	return false
}
