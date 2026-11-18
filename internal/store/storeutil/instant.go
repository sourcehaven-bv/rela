package storeutil

import (
	"regexp"
	"strings"
)

// Shared SQL text helpers for the point-in-time read seams of the database
// backends (TKT-6KLUT3U). Each backend prefixes its read statements with CTEs
// named after the live tables; these helpers do the text work.

// writeStatement matches a statement that modifies data: one that opens with
// a write keyword, one whose main statement follows a WITH list, and one with
// a data-modifying CTE (`WITH g AS (DELETE ... RETURNING *) SELECT ...`).
var writeStatement = regexp.MustCompile(
	`(?is)^\s*(insert|update|delete|replace|merge|truncate|copy|lock)\b` +
		`|\)\s*(insert|update|delete|replace|merge)\s` +
		`|\bas\s+(not\s+)?(materialized\s+)?\(\s*(insert|update|delete|merge)\b`)

// IsWriteStatement reports whether sql modifies data, by the shapes
// writeStatement names. It is a backstop behind a seam that already refuses
// its write entry points, not a parser: it does not see a write inside a
// function the statement calls, nor a locking read (SELECT ... FOR UPDATE).
func IsWriteStatement(sql string) bool { return writeStatement.MatchString(sql) }

// leadingWith matches a statement's own WITH list.
var leadingWith = regexp.MustCompile(`(?is)^\s*with(\s+recursive)?\s`)

// PrefixCTEs puts ctes (a comma-separated CTE list) in front of sql. A
// statement that opens with WITH or WITH RECURSIVE keeps its keyword, and its
// own CTEs follow the ones added.
func PrefixCTEs(sql, ctes string) string {
	m := leadingWith.FindStringSubmatchIndex(sql)
	if m == nil {
		return "WITH " + ctes + "\n" + sql
	}
	keyword := "WITH "
	if m[2] >= 0 {
		keyword = "WITH RECURSIVE "
	}
	return keyword + ctes + ",\n" + sql[m[1]:]
}

// liveMarker prefixes a table name in read SQL that must name the live table
// even under a read instant. Outside an instant it is a comment, so the name
// resolves to the live table as written. Under one the seam replaces it with
// a schema qualifier ([ResolveLive]), which no snapshot CTE can shadow.
const liveMarker = "/*rela:live*/"

// LiveTable renders table so that it reads the live rows under a read
// instant too. The ACL read gate uses it: history is read with today's ACL.
func LiveTable(table string) string { return liveMarker + table }

// ResolveLive qualifies every [LiveTable] name in sql with qualifier (an
// already quoted schema name), so it bypasses the snapshot CTEs.
func ResolveLive(sql, qualifier string) string {
	return strings.ReplaceAll(sql, liveMarker, qualifier+".")
}
