package repository

import (
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDB opens a pure-Go SQLite connection (no CGO) and runs schema migrations.
func NewDB(path string) (*gorm.DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening sqlite connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("getting raw sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := migrateDomainUsageSchema(db); err != nil {
		return nil, fmt.Errorf("migrating domain usage schema: %w", err)
	}

	if err := db.AutoMigrate(
		&nodeModel{},
		&nodeGroupModel{},
		&tokenModel{},
		&tokenGroupModel{},
		&tokenInboundModel{},
		&tokenIPRestrictionModel{},
		&groupModel{},
		&publicSourceModel{},
		&adminModel{},
		&inboundModel{},
		&tokenUsageModel{},
		&nodeUsageModel{},
		&inboundUsageModel{},
		&domainUsageModel{},
	); err != nil {
		// AutoMigrate handles new columns (is_self) automatically.
		return nil, fmt.Errorf("running sqlite migrations: %w", err)
	}

	if err := db.Exec("UPDATE tokens SET used_bytes = 0 WHERE used_bytes IS NULL").Error; err != nil {
		return nil, fmt.Errorf("initializing token used_bytes: %w", err)
	}

	return db, nil
}

// migrateDomainUsageSchema recreates the domain_usage table if node_id is missing
// from its composite primary key. SQLite cannot ALTER TABLE to add a PK column.
func migrateDomainUsageSchema(db *gorm.DB) error {
	var exists int64
	if err := db.Raw(
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='domain_usage'",
	).Scan(&exists).Error; err != nil {
		return fmt.Errorf("checking domain_usage existence: %w", err)
	}
	if exists == 0 {
		return nil
	}

	var pkCols int64
	if err := db.Raw(
		"SELECT count(*) FROM pragma_table_info('domain_usage') WHERE name = 'node_id' AND pk > 0",
	).Scan(&pkCols).Error; err != nil {
		return fmt.Errorf("checking domain_usage primary key: %w", err)
	}
	if pkCols > 0 {
		return nil
	}

	// Schema mismatch: recreate table with correct primary key.
	if err := db.Exec(`
		CREATE TABLE domain_usage_new (
			token_id TEXT,
			node_id TEXT,
			domain TEXT,
			period_type TEXT,
			period_start DATETIME,
			upload_bytes INTEGER,
			download_bytes INTEGER,
			updated_at DATETIME,
			PRIMARY KEY (token_id, node_id, domain, period_type, period_start)
		);
		INSERT INTO domain_usage_new
		SELECT token_id, COALESCE(node_id, ''), domain, period_type, period_start, upload_bytes, download_bytes, updated_at
		FROM domain_usage;
		DROP TABLE domain_usage;
		ALTER TABLE domain_usage_new RENAME TO domain_usage;
	`).Error; err != nil {
		return fmt.Errorf("recreating domain_usage table: %w", err)
	}
	return nil
}

func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
