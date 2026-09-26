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

// buildPostgresDSN constructs a lib/pq keyword/value connection string from the
// repository configuration.
//
// Every string value is single-quoted and escaped so it is a valid lib/pq
// keyword/value token: an empty value becomes an empty quoted string (which
// parseOpts reads as a clean empty string instead of folding the following
// key=value token into the empty value), and values containing whitespace,
// single quotes, or backslashes parse to their literal form instead of
// truncating or breaking the DSN. The integer port needs no quoting. See
// lib/pq's parseOpts for the quoting rules.
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

// pqQuote wraps s in single quotes and escapes backslashes and single quotes
// so the result is a valid lib/pq single-quoted value. lib/pq's parseOpts reads
// backslash-escaped characters inside single-quoted values (\\ -> \, \' -> ')
// and treats an unescaped single quote as the closing delimiter; unlike libpq
// it does not accept a doubled single-quote as an escaped quote, so backslash
// escaping is the only correct approach here.
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
