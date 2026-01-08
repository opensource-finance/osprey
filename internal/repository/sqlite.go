package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opensource-finance/osprey/internal/domain"
	_ "modernc.org/sqlite"
)

// openSQLite opens a SQLite database connection.
// Uses modernc.org/sqlite for pure Go implementation (no CGO required).
func openSQLite(cfg domain.RepositoryConfig) (*sql.DB, error) {
	path := cfg.SQLitePath
	if path == "" {
		path = "./osprey.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Build connection string with pragmas for performance
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	return db, nil
}
