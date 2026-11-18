package storeutil_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

func TestPrefixCTEs(t *testing.T) {
	tests := []struct{ name, sql, want string }{
		{"plain", "SELECT 1", "WITH a AS (x)\nSELECT 1"},
		{"with", "  with b AS (y) SELECT 1", "WITH a AS (x),\nb AS (y) SELECT 1"},
		{"recursive", "WITH RECURSIVE b AS (y) SELECT 1", "WITH RECURSIVE a AS (x),\nb AS (y) SELECT 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, storeutil.PrefixCTEs(tc.sql, "a AS (x)"))
		})
	}
}

func TestIsWriteStatement(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"SELECT * FROM entities", false},
		{"WITH x AS (SELECT 1) SELECT * FROM x", false},
		{"SELECT updated_at FROM entities", false},
		{" INSERT INTO entities VALUES (1)", true},
		{"update entities SET a = 1", true},
		{"DELETE FROM relations", true},
		{"REPLACE INTO entities VALUES (1)", true},
		{"WITH x AS (SELECT 1) DELETE FROM entities", true},
		{"WITH g AS (DELETE FROM entities RETURNING *) SELECT * FROM g", true},
		{"WITH g AS NOT MATERIALIZED ( update entities SET a = 1 RETURNING id) SELECT 1", true},
		{"WITH g AS MATERIALIZED (INSERT INTO entities VALUES (1) RETURNING id) SELECT 1", true},
		{"SELECT CAST(x AS text) FROM entities", false},
	}
	for _, tc := range tests {
		t.Run(tc.sql, func(t *testing.T) {
			require.Equal(t, tc.want, storeutil.IsWriteStatement(tc.sql))
		})
	}
}

func TestResolveLive(t *testing.T) {
	sql := "SELECT 1 FROM " + storeutil.LiveTable("relations") + " r JOIN entities e ON TRUE"
	require.Equal(t, `SELECT 1 FROM "s".relations r JOIN entities e ON TRUE`, storeutil.ResolveLive(sql, `"s"`))
}
