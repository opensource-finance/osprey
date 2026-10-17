package repository

import (
	"strings"
	"testing"
)

func TestTransformForPostgresRewritesBLOBToBYTEA(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "transactions schema BLOB becomes BYTEA",
			in:   schemaTransactions,
			want: strings.ReplaceAll(strings.ReplaceAll(schemaTransactions, " BLOB,", " BYTEA,"), " REAL ", " DOUBLE PRECISION "),
		},
		{
			name: "simple column declaration",
			in:   "CREATE TABLE t (id TEXT, data BLOB, PRIMARY KEY (id));",
			want: "CREATE TABLE t (id TEXT, data BYTEA, PRIMARY KEY (id));",
		},
		{
			name: "multiple BLOB columns all rewritten",
			in:   "CREATE TABLE t (a BLOB, b BLOB, c TEXT);",
			want: "CREATE TABLE t (a BYTEA, b BYTEA, c TEXT);",
		},
		{
			name: "schema with no BLOB and no REAL unchanged",
			in:   "CREATE TABLE t (id TEXT NOT NULL, enabled INTEGER NOT NULL);",
			want: "CREATE TABLE t (id TEXT NOT NULL, enabled INTEGER NOT NULL);",
		},
		{
			name: "empty string unchanged",
			in:   "",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := transformForPostgres(c.in)
			if got != c.want {
				t.Errorf("transformForPostgres() mismatch\n  got:  %s\n  want: %s", got, c.want)
			}
		})
	}
}

func TestTransformForPostgresOnlyMatchesBoundedPattern(t *testing.T) {
	// The bounded pattern " BLOB," must not corrupt a column NAMED blob_data,
	// a trailing BLOB without a comma, or a BLOB preceded by something other
	// than a space.
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "column named blob_data stays TEXT",
			in:   "CREATE TABLE t (blob_data TEXT, other BLOB, PRIMARY KEY (blob_data));",
			want: "CREATE TABLE t (blob_data TEXT, other BYTEA, PRIMARY KEY (blob_data));",
		},
		{
			name: "trailing BLOB without comma is not rewritten (last column)",
			in:   "CREATE TABLE t (id TEXT, data BLOB)",
			want: "CREATE TABLE t (id TEXT, data BLOB)",
		},
		{
			name: "BLOB in a comment is not rewritten (no space-comma pattern)",
			in:   "-- this column stores a BLOB\nCREATE TABLE t (id TEXT);",
			want: "-- this column stores a BLOB\nCREATE TABLE t (id TEXT);",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := transformForPostgres(c.in)
			if got != c.want {
				t.Errorf("transformForPostgres() mismatch\n  got:  %s\n  want: %s", got, c.want)
			}
		})
	}
}

func TestAllSchemasHaveExactlyOneBLOBDeclaration(t *testing.T) {
	// Guards against drift: if a second BLOB is added to the schemas, this test
	// forces the author to confirm the bounded replacement still handles it.
	var count int
	for _, schema := range AllSchemas() {
		count += strings.Count(schema, " BLOB,")
	}
	if count != 1 {
		t.Fatalf("expected exactly one \" BLOB,\" declaration across AllSchemas(), got %d", count)
	}
}

func TestTransformForPostgresRewritesREALToDoublePrecision(t *testing.T) {
	// PostgreSQL REAL is float4 and rounds 1234567.89 to 1234567.9 (issue #65).
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "transactions.amount",
			in:   "    amount REAL NOT NULL,",
			want: "    amount DOUBLE PRECISION NOT NULL,",
		},
		{
			name: "REAL with a default",
			in:   "    weight REAL NOT NULL DEFAULT 1.0,",
			want: "    weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,",
		},
		{
			name: "REAL as the last column",
			in:   "CREATE TABLE t (id TEXT, amount REAL)",
			want: "CREATE TABLE t (id TEXT, amount DOUBLE PRECISION)",
		},
		{
			name: "REAL followed by a comma",
			in:   "CREATE TABLE t (amount REAL, id TEXT)",
			want: "CREATE TABLE t (amount DOUBLE PRECISION, id TEXT)",
		},
		{
			name: "column names containing real are preserved",
			in:   "CREATE TABLE t (real_amount TEXT, XREAL TEXT, unreal REAL NOT NULL);",
			want: "CREATE TABLE t (real_amount TEXT, XREAL TEXT, unreal DOUBLE PRECISION NOT NULL);",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := transformForPostgres(c.in)
			if got != c.want {
				t.Errorf("transformForPostgres() mismatch\n  got:  %s\n  want: %s", got, c.want)
			}
		})
	}
}

func TestAllSchemasHaveNoREALAfterPostgresTransform(t *testing.T) {
	// Every REAL column (amount, weight, score, alert_threshold) must become
	// float8 on PostgreSQL, including any REAL column added later.
	var before, after int
	for _, schema := range AllSchemas() {
		before += len(realType.FindAllString(schema, -1))
		after += len(realType.FindAllString(transformForPostgres(schema), -1))
	}
	if before == 0 {
		t.Fatal("expected REAL declarations in AllSchemas(); the drift guard is checking nothing")
	}
	if after != 0 {
		t.Fatalf("%d REAL declarations survive transformForPostgres", after)
	}
}
