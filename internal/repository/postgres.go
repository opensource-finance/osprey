package repository

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	"github.com/opensource-finance/osprey/internal/domain"
)

// openPostgres opens a PostgreSQL database connection.
func openPostgres(cfg domain.RepositoryConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", buildPostgresDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	return db, nil
}

// buildPostgresDSN builds a lib/pq DSN with every string value quoted, so empty
// or whitespace credentials parse correctly.
func buildPostgresDSN(cfg domain.RepositoryConfig) string {
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

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		pqQuote(host),
		port,
		pqQuote(cfg.PostgresUser),
		pqQuote(cfg.PostgresPassword),
		pqQuote(dbname),
		pqQuote(getSSLMode(cfg.PostgresSSLMode)),
	)
}

// pqQuote single-quotes s for lib/pq; it needs backslash escapes, not doubled quotes.
func pqQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}

func getSSLMode(mode string) string {
	if mode == "" {
		return "disable"
	}
	return mode
}
