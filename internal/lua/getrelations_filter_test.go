package lua

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// relationFilterFixture seeds a graph where a filtered and an unfiltered
// rela.get_relations return DIFFERENT counts. The single-relation default
// fixture cannot distinguish "filtered to nothing" from "returned everything",
// which is the exact confusion this file exists to pin.
func relationFilterFixture(t *testing.T) *mockWorkspace {
	t.Helper()
	m := newMockWorkspaceWith(testMeta())
	m.seedEntity(&entity.Entity{ID: "TKT-001", Type: "ticket"})
	m.seedEntity(&entity.Entity{ID: "TKT-002", Type: "ticket"})
	m.seedEntity(&entity.Entity{ID: "FEAT-001", Type: "feature"})
	m.seedRelation(&entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
	m.seedRelation(&entity.Relation{From: "TKT-002", Type: "implements", To: "FEAT-001"})
	return m
}

// runRelationScript executes src against the fixture and returns the decoded
// rela.output payload.
func runRelationScript(t *testing.T, src string) (map[string]any, error) {
	t.Helper()
	ws := relationFilterFixture(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()

	if err := r.RunString(src); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("decode output %q: %v", buf.String(), err)
	}
	return result, nil
}

// TestGetRelations_RejectsNonStringFilter pins that a mistyped filter on the
// GATED rela.get_relations fails loudly rather than silently widening to a
// whole-graph edge dump (RR-D7KXKV).
//
// The failure this prevents is quiet and plausible: `{from = some_id}` where
// some_id arrived as a number returns EVERY edge the caller may see, and the
// script consumes that as if it were the edges of one entity. Peer-gating
// bounds the disclosure to the caller's own view — this is not an ACL hole —
// but it silently turns a scoped question into an unscoped one, which is a
// correctness bug in every script that asks it.
func TestGetRelations_RejectsNonStringFilter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, opts string }{
		{"numeric from", `{from = 12345}`},
		{"boolean type", `{type = true}`},
		{"table to", `{to = {}}`},
		{"numeric to", `{to = 0}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runRelationScript(t,
				`rela.output({count = #rela.get_relations(`+tc.opts+`)})`)
			if err == nil {
				t.Fatal("a non-string filter was silently DROPPED -- the call " +
					"returned an unfiltered edge dump that the script reads as filtered")
			}
			if !strings.Contains(err.Error(), "must be a string") {
				t.Errorf("error = %v, want it to name the bad option type", err)
			}
		})
	}
}

// TestGetRelations_AcceptsAbsentAndStringFilters pins that the rejection above
// did not turn "absent" into "invalid". Omitting the table, passing an empty
// one, or naming only some keys all remain valid and mean "no constraint on
// that field" — the behavior every in-tree caller relies on.
func TestGetRelations_AcceptsAbsentAndStringFilters(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		call  string
		count float64
	}{
		{"no argument", `rela.get_relations()`, 2},
		{"empty table", `rela.get_relations({})`, 2},
		{"from filter", `rela.get_relations({from = "TKT-001"})`, 1},
		{"type filter", `rela.get_relations({type = "implements"})`, 2},
		{"to filter", `rela.get_relations({to = "FEAT-001"})`, 2},
		{"no match", `rela.get_relations({from = "TKT-404"})`, 0},
		{"explicit nil", `rela.get_relations({from = nil, type = "implements"})`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := runRelationScript(t,
				`rela.output({count = #`+tc.call+`})`)
			if err != nil {
				t.Fatalf("valid filter was rejected: %v", err)
			}
			if got := out["count"]; got != tc.count {
				t.Errorf("count = %v, want %v -- the filter did not scope the result "+
					"as documented", got, tc.count)
			}
		})
	}
}

// TestGetRelations_NonTableArgumentIsIgnored pins the pre-existing contract for
// a non-table first argument: it is ignored, yielding the unfiltered result.
//
// This is deliberately NOT tightened alongside the option types. `{from = 12345}`
// is a mistake with no plausible intent; `rela.get_relations("x")` reaches a
// documented "opts is optional" path, and raising there would break callers
// that pass a stray value through a wrapper. Pinning it here makes the
// asymmetry a decision rather than an oversight.
func TestGetRelations_NonTableArgumentIsIgnored(t *testing.T) {
	t.Parallel()

	out, err := runRelationScript(t, `rela.output({count = #rela.get_relations("nonsense")})`)
	if err != nil {
		t.Fatalf("non-table argument raised: %v", err)
	}
	if got := out["count"]; got != float64(2) {
		t.Errorf("count = %v, want 2 (unfiltered)", got)
	}
}
