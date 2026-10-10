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
			want: strings.ReplaceAll(schemaTransactions, " BLOB,", " BYTEA,"),
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
			name: "rule_configs schema unchanged (no BLOB)",
			in:   schemaRuleConfigs,
			want: schemaRuleConfigs,
		},
		{
			name: "evaluations schema unchanged (no BLOB)",
			in:   schemaEvaluations,
			want: schemaEvaluations,
		},
		{
			name: "typologies schema unchanged (no BLOB)",
			in:   schemaTypologies,
			want: schemaTypologies,
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
