package repository

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/opensource-finance/osprey/internal/domain"
)

func TestPQQuote(t *testing.T) {
	// pqQuote must produce a lib/pq single-quoted value where empty stays
	// empty, whitespace is preserved, and backslash/single-quote are
	// backslash-escaped. lib/pq's parseOpts does not accept '' doubling, so
	// backslash escaping is the only correct form.
	cases := []struct {
		in   string
		want string
	}{
		{``, `''`},
		{`plain`, `'plain'`},
		{`with spaces`, `'with spaces'`},
		{` leading and trailing `, `' leading and trailing '`},
		{`with'quote`, `'with\'quote'`},
		{`with\backslash`, `'with\\backslash'`},
		{`'leading`, `'\'leading'`},
		{`trailing\`, `'trailing\\'`},
		{`'both\`, `'\'both\\'`},
	}
	for _, c := range cases {
		got := pqQuote(c.in)
		if got != c.want {
			t.Errorf("pqQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildPostgresDSN(t *testing.T) {
	// Every string field is single-quoted/escaped. Empty fields become '' so
	// lib/pq parses them as clean empty values and does not fold the following
	// key=value token into the empty value. Whitespace and special characters
	// in values are preserved verbatim through quoting.
	cases := []struct {
		name string
		cfg  domain.RepositoryConfig
		want string
	}{
		{
			name: "all fields set, no special characters",
			cfg: domain.RepositoryConfig{
				PostgresHost: "db.example.com", PostgresPort: 6432,
				PostgresUser: "osprey", PostgresPassword: "s3cret",
				PostgresDB: "ospreydb", PostgresSSLMode: "require",
			},
			want: `host='db.example.com' port=6432 user='osprey' password='s3cret' dbname='ospreydb' sslmode='require'`,
		},
		{
			name: "defaults applied for host port and dbname",
			cfg: domain.RepositoryConfig{
				PostgresUser: "u", PostgresPassword: "p",
			},
			want: `host='localhost' port=5432 user='u' password='p' dbname='osprey' sslmode='disable'`,
		},
		{
			name: "empty user does not fold the following token (E1b)",
			cfg: domain.RepositoryConfig{
				PostgresUser: "", PostgresPassword: "secret", PostgresDB: "osprey",
			},
			want: `host='localhost' port=5432 user='' password='secret' dbname='osprey' sslmode='disable'`,
		},
		{
			name: "empty password does not fold the following token (E1a)",
			cfg: domain.RepositoryConfig{
				PostgresUser: "postgres", PostgresPassword: "", PostgresDB: "osprey",
			},
			want: `host='localhost' port=5432 user='postgres' password='' dbname='osprey' sslmode='disable'`,
		},
		{
			name: "both credentials empty, pro defaults (E2)",
			cfg: domain.RepositoryConfig{
				PostgresUser: "", PostgresPassword: "", PostgresDB: "osprey",
			},
			want: `host='localhost' port=5432 user='' password='' dbname='osprey' sslmode='disable'`,
		},
		{
			name: "password with interior whitespace is preserved (E3a)",
			cfg: domain.RepositoryConfig{
				PostgresUser: "postgres", PostgresPassword: " secret with spaces ", PostgresDB: "osprey",
			},
			want: `host='localhost' port=5432 user='postgres' password=' secret with spaces ' dbname='osprey' sslmode='disable'`,
		},
		{
			name: "password with leading single quote is escaped (E3b)",
			cfg: domain.RepositoryConfig{
				PostgresUser: "postgres", PostgresPassword: "'Brien", PostgresDB: "osprey",
			},
			want: `host='localhost' port=5432 user='postgres' password='\'Brien' dbname='osprey' sslmode='disable'`,
		},
		{
			name: "password with trailing backslash is escaped (E3c)",
			cfg: domain.RepositoryConfig{
				PostgresUser: "postgres", PostgresPassword: `pass\`, PostgresDB: "osprey",
			},
			want: `host='localhost' port=5432 user='postgres' password='pass\\' dbname='osprey' sslmode='disable'`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildPostgresDSN(c.cfg)
			if got != c.want {
				t.Errorf("buildPostgresDSN() =\n  %s\nwant\n  %s", got, c.want)
			}
		})
	}
}

// TestOpenPostgresStartupPacketFields drives openPostgres end-to-end against a
// mock PostgreSQL server that captures the v3 startup packet lib/pq sends on
// db.Ping(). The startup packet carries the "user" and "database" lib/pq parsed
// out of the DSN, so it proves three things at once:
//
//  1. The DSN parsed without error (a parse error sends no packet at all).
//  2. Empty credentials do not fold the following token into the value - the
//     startup packet's user field is the intended empty string, not the literal
//     "password=..." token that the pre-fix unquoted DSN produced.
//  3. Whitespace- and special-character-bearing passwords no longer prevent the
//     connection from reaching the server.
//
// The mock never completes authentication, so openPostgres always returns an
// error here; only the captured packet matters.
func TestOpenPostgresStartupPacketFields(t *testing.T) {
	cases := []struct {
		name     string
		user     string
		password string
		wantUser string
		wantDB   string
	}{
		{"happy path", "postgres", "secret", "postgres", "osprey"},
		{"E1b user forgotten password set", "", "secret", "", "osprey"},
		{"E2 both credentials empty", "", "", "", "osprey"},
		{"E3a password with interior whitespace", "postgres", " secret with spaces ", "postgres", "osprey"},
		{"E3b password with leading single quote", "postgres", "'Brien", "postgres", "osprey"},
		{"E3c password with trailing backslash", "postgres", `pass\`, "postgres", "osprey"},
		{"E3 combined quote and backslash", "postgres", `'both\`, "postgres", "osprey"},
		{"custom database name preserved", "svc", "p", "svc", "custom-db"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mock := newMockPostgres(t)
			defer mock.close()

			host, portStr, err := net.SplitHostPort(mock.addr())
			if err != nil {
				t.Fatalf("split mock addr: %v", err)
			}
			port, err := strconv.Atoi(portStr)
			if err != nil {
				t.Fatalf("parse mock port: %v", err)
			}

			cfg := domain.RepositoryConfig{
				Driver:           "postgres",
				PostgresHost:     host,
				PostgresPort:     port,
				PostgresUser:     c.user,
				PostgresPassword: c.password,
				PostgresDB:       c.wantDB,
				// sslmode unset -> getSSLMode returns "disable", so lib/pq skips
				// SSL negotiation and the first bytes on the wire are the
				// startup packet the mock reads.
			}

			if _, err := openPostgres(cfg); err == nil {
				t.Fatalf("expected openPostgres to fail against the mock (it never completes auth)")
			}

			opts := mock.waitForPacket(5 * time.Second)
			if opts == nil {
				t.Fatalf("no startup packet received from openPostgres; DSN may have failed to parse")
			}
			if got := opts["user"]; got != c.wantUser {
				t.Errorf("startup packet user = %q, want %q (fold would surface the following token here)", got, c.wantUser)
			}
			if got := opts["database"]; got != c.wantDB {
				t.Errorf("startup packet database = %q, want %q", got, c.wantDB)
			}
		})
	}
}

type mockPostgres struct {
	listener net.Listener
	opts     map[string]string
	done     chan struct{}
	once     sync.Once
}

func newMockPostgres(t *testing.T) *mockPostgres {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	m := &mockPostgres{
		listener: l,
		opts:     nil,
		done:     make(chan struct{}),
	}
	go m.serve()
	return m
}

func (m *mockPostgres) addr() string { return m.listener.Addr().String() }

func (m *mockPostgres) close() { _ = m.listener.Close() }

func (m *mockPostgres) serve() {
	defer m.once.Do(func() { close(m.done) })

	conn, err := m.listener.Accept()
	if err != nil {
		return // listener closed before a connection arrived (timeout path)
	}
	defer func() { _ = conn.Close() }()

	opts, err := readPostgresStartupPacket(conn)
	if err != nil {
		return
	}
	m.opts = opts
}

func (m *mockPostgres) waitForPacket(timeout time.Duration) map[string]string {
	select {
	case <-m.done:
	case <-time.After(timeout):
		_ = m.listener.Close() // unblock Accept so serve exits and done closes
		<-m.done
	}
	return m.opts
}

// readPostgresStartupPacket reads a PostgreSQL v3 startup message from conn and
// returns its key/value parameters. A v3 startup message is: uint32 total
// length (including the length field), uint32 protocol version (196608), then
// NUL-terminated key/value string pairs terminated by an empty key.
func readPostgresStartupPacket(conn net.Conn) (map[string]string, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen < 8 || msgLen > 1<<20 {
		return nil, fmt.Errorf("unexpected startup packet length %d", msgLen)
	}
	body := make([]byte, msgLen-4)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}

	kv := body[4:] // skip 4-byte protocol version
	opts := map[string]string{}
	for len(kv) > 0 {
		key, rest, ok := readCString(kv)
		if !ok || key == "" {
			break // trailing zero byte terminates the parameter list
		}
		kv = rest
		val, rest, ok := readCString(kv)
		if !ok {
			break
		}
		kv = rest
		opts[key] = val
	}
	return opts, nil
}

func readCString(b []byte) (string, []byte, bool) {
	for i, c := range b {
		if c == 0 {
			return string(b[:i]), b[i+1:], true
		}
	}
	return "", nil, false
}
