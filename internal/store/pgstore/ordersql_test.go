//go:build postgres

package pgstore

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// An expression index is matched by expression EQUIVALENCE, so the rank in a
// query's ORDER BY and the rank in the index DDL have to be the same
// expression. Nothing else fails when they drift: the index is created,
// maintained on every write, and silently never used.
//
// These tests need no database. The EXPLAIN test (derivedschema_explain_test.go)
// proves the planner agrees; these prove the two generators agree, which is the
// part a refactor breaks.
func TestOrderSQL_IndexRankAppearsVerbatimInQuery(t *testing.T) {
	values := []string{"todo", "doing", "blocked", "done"}
	query := orderKeySQL(&sqlBuilder{}, store.OrderSpec{Property: "status", Values: values})
	index := orderRankSQL("status", values)

	if index == "" {
		t.Fatal("declared values produced no index rank expression")
	}
	if !strings.Contains(query, index) {
		t.Errorf("index rank is not a substring of the query rank\n  query: %s\n  index: %s", query, index)
	}
}

// The property name is interpolated, not bound, for a ranked key. Measured on
// 200k rows under a forced generic plan: `properties ->> $n` seq-scans at 1,915
// buffers because it is not the same expression as `properties->>'status'`,
// while the literal form is an Index Scan at 4 buffers (TKT-9OFGH4).
func TestOrderSQL_RankedKeyInterpolatesPropertyName(t *testing.T) {
	query := orderKeySQL(&sqlBuilder{}, store.OrderSpec{
		Property: "status", Values: []string{"todo", "done"},
	})
	if strings.Contains(query, "$") {
		t.Errorf("ranked key must not carry a bind parameter, got %s", query)
	}
	if !strings.Contains(query, "'status'") {
		t.Errorf("ranked key should interpolate the property name, got %s", query)
	}
}

// An unranked key keeps its bind parameter: there is no expression index to
// match, so nothing is gained by interpolating and the parameter is safer.
func TestOrderSQL_UnrankedKeyKeepsBindParameter(t *testing.T) {
	query := orderKeySQL(&sqlBuilder{}, store.OrderSpec{Property: "title"})
	if !strings.Contains(query, "$") {
		t.Errorf("unranked key should bind the property name, got %s", query)
	}
}

// Direction applies to the rank AND the text tiebreak. Ranking ascending while
// the tiebreak descends would order the enum forwards and its undeclared
// values backwards.
func TestOrderSQL_DirectionAppliesToBothKeys(t *testing.T) {
	query := orderKeySQL(&sqlBuilder{}, store.OrderSpec{
		Property: "status", Values: []string{"todo", "done"}, Descending: true,
	})
	if got := strings.Count(query, " DESC"); got != 2 {
		t.Errorf("expected DESC on both the rank and the text key, got %d in %s", got, query)
	}
	if strings.Contains(query, " ASC") {
		t.Errorf("descending spec emitted an ASC key: %s", query)
	}
}

// A value containing a quote must not break the statement. Operator-authored
// config is trusted, but a typo should surface as a value that sorts oddly, not
// as a syntax error at query time.
func TestOrderSQL_EscapesQuotesInValues(t *testing.T) {
	query := orderKeySQL(&sqlBuilder{}, store.OrderSpec{
		Property: "status", Values: []string{"it's"},
	})
	if !strings.Contains(query, "'it''s'") {
		t.Errorf("embedded quote not doubled: %s", query)
	}
}

// Reordering `values:` must RENAME the index, so the reconciler drops the old
// one instead of keeping an index that ranks by the previous positions. Same
// mechanism as uniqueIndexShape (BUG-HC6I2T).
func TestListIndexName_ChangesWithDeclaredValueOrder(t *testing.T) {
	base := store.DerivedObjectSpec{
		Kind: store.DerivedListIndex, Type: "ticket", OrderBy: []string{"status"},
	}
	withValues := base
	withValues.OrderValues = [][]string{{"todo", "doing", "done"}}
	reordered := base
	reordered.OrderValues = [][]string{{"doing", "todo", "done"}}
	inserted := base
	inserted.OrderValues = [][]string{{"todo", "doing", "review", "done"}}

	names := map[string]string{
		"no values": listIndexName(base),
		"declared":  listIndexName(withValues),
		"reordered": listIndexName(reordered),
		"inserted":  listIndexName(inserted),
	}
	seen := map[string]string{}
	for label, name := range names {
		if other, dup := seen[name]; dup {
			t.Errorf("%q and %q share an index name (%s); a schema edit would keep a stale index",
				label, other, name)
		}
		seen[name] = label
	}
}
