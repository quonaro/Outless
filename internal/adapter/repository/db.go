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

	if err := db.AutoMigrate(
		&nodeModel{},
		&tokenModel{},
		&tokenGroupModel{},
		&groupModel{},
		&publicSourceModel{},
		&adminModel{},
		&inboundModel{},
	); err != nil {
		return nil, fmt.Errorf("running sqlite migrations: %w", err)
	}

	return db, nil
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
