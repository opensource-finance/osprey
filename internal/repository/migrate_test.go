package repository

import (
	"strings"
	"testing"
)

func TestTransformForPostgresRewritesBLOBToBYTEA(t *testing.T) {
	// Inputs here deliberately contain no REAL tokens so the BLOB rewrite is
	// observed in isolation.
	cases := []struct {
		name string
		in   string
		want string
	}{
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
			name: "schema with no BLOB and no REAL is unchanged",
			in:   "CREATE TABLE t (id TEXT NOT NULL, name TEXT NOT NULL);",
			want: "CREATE TABLE t (id TEXT NOT NULL, name TEXT NOT NULL);",
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

func TestTransformForPostgresOnlyMatchesBoundedBLOBPattern(t *testing.T) {
	// The bounded pattern " BLOB," must not corrupt a column NAMED blob_data,
	// a trailing BLOB without a comma, or prose (no space-comma pattern).
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

func TestTransformForPostgresRewritesREALInEverySchema(t *testing.T) {
	// Each schema that declares a REAL column must have it rewritten to
	// DOUBLE PRECISION, and the value-magnitude columns must specifically be
	// float8 on PostgreSQL.
	cases := []struct {
		name   string
		schema string
		// columnLine is the exact SQLite-dialect line that should appear in the
		// source schema (and is what the bug corrupts on PostgreSQL).
		columnLine string
		// wantLine is the exact line expected after the Postgres transform.
		wantLine string
	}{
		{
			name:       "transactions.amount",
			schema:     schemaTransactions,
			columnLine: "    amount REAL NOT NULL,",
			wantLine:   "    amount DOUBLE PRECISION NOT NULL,",
		},
		{
			name:       "rule_configs.weight",
			schema:     schemaRuleConfigs,
			columnLine: "    weight REAL NOT NULL DEFAULT 1.0,",
			wantLine:   "    weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,",
		},
		{
			name:       "evaluations.score",
			schema:     schemaEvaluations,
			columnLine: "    score REAL NOT NULL,",
			wantLine:   "    score DOUBLE PRECISION NOT NULL,",
		},
		{
			name:       "typologies.alert_threshold",
			schema:     schemaTypologies,
			columnLine: "    alert_threshold REAL NOT NULL DEFAULT 0.6,",
			wantLine:   "    alert_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.6,",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(c.schema, c.columnLine) {
				t.Fatalf("source schema no longer declares %q (schema drift); expected line: %s", c.name, c.columnLine)
			}
			got := transformForPostgres(c.schema)
			if !strings.Contains(got, c.wantLine) {
				t.Errorf("Postgres transform did not rewrite %q to %q\n  got: %s", c.columnLine, c.wantLine, got)
			}
			if strings.Contains(got, c.columnLine) {
				t.Errorf("Postgres transform left the SQLite REAL line in place: %q\n  got: %s", c.columnLine, got)
			}
			if strings.Contains(got, " REAL ") {
				t.Errorf("Postgres transform left a \" REAL \" token in %q\n  got: %s", c.name, got)
			}
		})
	}
}

func TestTransformForPostgresOnlyMatchesBoundedREALPattern(t *testing.T) {
	// The bounded pattern " REAL " targets the column-type token. It must not
	// rewrite a column NAME that merely contains "real", nor a REAL token that
	// lacks the bounding spaces (e.g. a last column terminated by a paren).
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "column name containing lowercase real is preserved; only the type token is rewritten",
			in:   "CREATE TABLE t (real_amount TEXT, amount REAL NOT NULL);",
			want: "CREATE TABLE t (real_amount TEXT, amount DOUBLE PRECISION NOT NULL);",
		},
		{
			name: "uppercase REAL without a leading space (part of a name) is preserved",
			in:   "CREATE TABLE t (id TEXT, XREAL TEXT, amount REAL NOT NULL);",
			want: "CREATE TABLE t (id TEXT, XREAL TEXT, amount DOUBLE PRECISION NOT NULL);",
		},
		{
			name: "trailing REAL without a trailing space (last column) is not rewritten",
			in:   "CREATE TABLE t (id TEXT, amount REAL)",
			want: "CREATE TABLE t (id TEXT, amount REAL)",
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

func TestAllSchemasHaveExpectedREALDeclarationsAndAllAreRewritten(t *testing.T) {
	// Drift guard for the float4/float8 bug: count every REAL column-type token
	// in the shared DDL. If a new REAL column is added, this test forces the
	// author to confirm transformForPostgres still rewrites it. The known
	// declarations are amount, weight, score, and alert_threshold.
	const expectedREALDecl = 4

	var realCount int
	for _, schema := range AllSchemas() {
		realCount += strings.Count(schema, " REAL ")
	}
	if realCount != expectedREALDecl {
		t.Fatalf("expected exactly %d \" REAL \" declarations across AllSchemas(), got %d", expectedREALDecl, realCount)
	}

	// After the Postgres transform, every REAL declaration must be gone and
	// replaced by exactly as many DOUBLE PRECISION declarations.
	var dpCount int
	var realRemaining int
	for _, schema := range AllSchemas() {
		transformed := transformForPostgres(schema)
		realRemaining += strings.Count(transformed, " REAL ")
		dpCount += strings.Count(transformed, " DOUBLE PRECISION ")
	}
	if realRemaining != 0 {
		t.Errorf("expected no \" REAL \" tokens to survive transformForPostgres, got %d", realRemaining)
	}
	if dpCount != expectedREALDecl {
		t.Errorf("expected %d \" DOUBLE PRECISION \" tokens after transform, got %d", expectedREALDecl, dpCount)
	}
}
