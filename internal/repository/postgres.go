package repository

import (
	"database/sql"
	"fmt"

	"github.com/opensource-finance/osprey/internal/domain"
	_ "github.com/lib/pq"
)

// openPostgres opens a PostgreSQL database connection.
func openPostgres(cfg domain.RepositoryConfig) (*sql.DB, error) {
	host := cfg.PostgresHost
	if host == "" {
		host = "localhost"
	}

	port := cfg.PostgresPort
	if port == 0 {
		port = 5432
	}

	dbname := cfg.PostgresDB
	if dbname == "" {
		dbname = "osprey"
	}

	// Build connection string
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host,
		port,
		cfg.PostgresUser,
		cfg.PostgresPassword,
		dbname,
		getSSLMode(cfg.PostgresSSLMode),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	return db, nil
}

func getSSLMode(mode string) string {
	if mode == "" {
		return "disable"
	}
	return mode
}
