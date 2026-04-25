package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// Migrator handles database schema migrations.
type Migrator struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewMigrator creates a new migrator instance.
func NewMigrator(db *sql.DB, logger *slog.Logger) *Migrator {
	return &Migrator{
		db:     db,
		logger: logger,
	}
}

// Up applies all pending migrations.
func (m *Migrator) Up(ctx context.Context) error {
	// Ensure schema_migrations table exists
	if err := m.createSchemaTable(ctx); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	// Get list of migration files
	files, err := FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("reading migration files: %w", err)
	}

	// Sort files by name
	var migrationFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			migrationFiles = append(migrationFiles, f.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Get applied migrations
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	// Apply pending migrations
	for _, file := range migrationFiles {
		version := strings.TrimSuffix(file, ".sql")
		if applied[version] {
			m.logger.Debug("migration already applied", slog.String("version", version))
			continue
		}

		m.logger.Info(fmt.Sprintf("Running migration: %s", file))
		if err := m.applyMigration(ctx, file, version); err != nil {
			return fmt.Errorf("applying migration %s: %w", version, err)
		}
		m.logger.Info(fmt.Sprintf("Migration applied: %s", file))
	}

	m.logger.Info("Migrations applied successfully", slog.Int("total", len(migrationFiles)))
	return nil
}

func (m *Migrator) createSchemaTable(ctx context.Context) error {
	// Create table if not exists
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	_, err := m.db.ExecContext(ctx, query)
	return err
}

func (m *Migrator) getAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	query := `SELECT version FROM schema_migrations;`
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func (m *Migrator) applyMigration(ctx context.Context, file, version string) error {
	// Read migration file
	content, err := FS.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading migration file %s: %w", file, err)
	}

	// Execute migration (ignore "already exists" errors)
	if _, err := m.db.ExecContext(ctx, string(content)); err != nil {
		// Ignore errors for objects that already exist
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "duplicate") {
			m.logger.Debug("migration object already exists, skipping", slog.String("file", file))
		} else {
			return fmt.Errorf("executing migration: %w", err)
		}
	}

	// Record migration in separate transaction
	insertQuery := `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2);`
	if _, err := m.db.ExecContext(ctx, insertQuery, version, time.Now().UTC()); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	return nil
}
