package lua

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// countingReader yields n synthetic entities and records how many the binding
// actually pulled. The pull count is the point: the bound must STOP the
// iterator, not slice a fully-materialized set.
type countingReader struct {
	EntityReader
	n      int
	pulled int
	failAt int // when > 0, yield an error after this many rows
}

func (c *countingReader) ListEntities(
	context.Context, store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for i := range c.n {
			if c.failAt > 0 && i == c.failAt {
				yield(nil, errStubRead)
				return
			}
			c.pulled++
			e := &entity.Entity{
				ID:         "TKT-" + string(rune('A'+i%26)) + string(rune('a'+i/26)),
				Type:       "ticket",
				Properties: map[string]any{"status": "open"},
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

func (c *countingReader) GetEntity(context.Context, string) (*entity.Entity, error) {
	return nil, errStubRead
}

func (c *countingReader) ListRelations(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(func(*entity.Relation, error) bool) {}
}

// runList executes src against a runtime whose reader yields n rows.
func runList(t *testing.T, rd *countingReader, src string) (map[string]any, error) {
	t.Helper()
	ws := newMockWorkspaceWith(testMeta())
	deps := ws.services("/tmp")
	deps.VisibleReader = rd

	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	defer r.Close()

	if err := r.RunString(src); err != nil {
		return nil, err
	}
	var out map[string]any
	if buf.Len() > 0 {
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("decode %q: %v", buf.String(), err)
		}
	}
	return out, nil
}

// TestListEntities_BoundedByDefault pins the core of DEC-IYHLNF: there is no
// unbounded read. A caller that names no limit still gets at most the ceiling.
// Not parallel: mutates the package-level ceiling (see the note on
// TestListEntities_LimitLowersTheBound).
func TestListEntities_BoundedByDefault(t *testing.T) {
	defer swapMaxReadLimit(t, 10)()

	rd := &countingReader{n: 50}
	out, err := runList(t, rd, `rela.output({n = #rela.list_entities("ticket")})`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := out["n"]; got != float64(10) {
		t.Errorf("rows = %v, want the ceiling 10 — an unbounded default is the defect", got)
	}
	// The bound must stop the ITERATOR. Materializing 50 and slicing to 10
	// would produce an identical Lua result while doing 5x the store work.
	if rd.pulled > 10 {
		t.Errorf("pulled %d rows for a limit of 10 — the bound must stop the iterator, not slice after", rd.pulled)
	}
}

// TestListEntities_LimitLowersTheBound pins that an explicit limit works and
// still cannot exceed the ceiling.
// NOT parallel, and neither are its subtests: they mutate the package-level
// ceiling. A t.Parallel() subtest resumes AFTER the parent returns, so the
// deferred restore would land first and the subtests would silently assert
// against the real 2000 — which is exactly how the clamp case first "passed".
func TestListEntities_LimitLowersTheBound(t *testing.T) {
	defer swapMaxReadLimit(t, 10)()

	for _, tc := range []struct {
		name string
		call string
		want float64
	}{
		{"below ceiling", `rela.list_entities("ticket", {limit = 3})`, 3},
		{"at ceiling", `rela.list_entities("ticket", {limit = 10})`, 10},
		{"above ceiling clamps", `rela.list_entities("ticket", {limit = 999})`, 10},
		{"limit exceeds available rows", `rela.list_entities("ticket", {limit = 10})`, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runList(t, &countingReader{n: 50}, `rela.output({n = #`+tc.call+`})`)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := out["n"]; got != tc.want {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestListEntities_RaisesOnIteratorError is the TKT-FVQ4 regression test.
// Before this, the binding broke out of the loop and returned the rows it had
// — a short list indistinguishable from a genuinely short result.
func TestListEntities_RaisesOnIteratorError(t *testing.T) {
	t.Parallel()
	rd := &countingReader{n: 50, failAt: 3}
	_, err := runList(t, rd, `rela.output({n = #rela.list_entities("ticket")})`)
	if err == nil {
		t.Fatal("a mid-iteration store error was SWALLOWED; the script received a " +
			"truncated list with no indication anything failed")
	}
	if !strings.Contains(err.Error(), "list_entities") {
		t.Errorf("error = %v, want it to name the failing binding", err)
	}
}

// TestListEntities_LegacyFilterStringStillWorks pins that the positional
// filter shorthand survives — it is the most common call in the tree.
func TestListEntities_LegacyFilterStringStillWorks(t *testing.T) {
	t.Parallel()
	out, err := runList(t, &countingReader{n: 5},
		`rela.output({n = #rela.list_entities("ticket", "status=open")})`)
	if err != nil {
		t.Fatalf("the bare filter string was rejected: %v", err)
	}
	if got := out["n"]; got != float64(5) {
		t.Errorf("rows = %v, want 5", got)
	}
}

// TestListEntities_FilterViaOptsEquivalentToString pins that the two spellings
// mean the same thing, so the shorthand is brevity and never semantics.
func TestListEntities_FilterViaOptsEquivalentToString(t *testing.T) {
	t.Parallel()
	viaString, err := runList(t, &countingReader{n: 5},
		`rela.output({n = #rela.list_entities("ticket", "status=open")})`)
	if err != nil {
		t.Fatalf("string form: %v", err)
	}
	viaOpts, err := runList(t, &countingReader{n: 5},
		`rela.output({n = #rela.list_entities("ticket", {filter = "status=open"})})`)
	if err != nil {
		t.Fatalf("opts form: %v", err)
	}
	if viaString["n"] != viaOpts["n"] {
		t.Errorf("string form gave %v, opts form gave %v — the spellings must agree",
			viaString["n"], viaOpts["n"])
	}
}

// TestListEntities_RejectsBadOptions pins that malformed options raise rather
// than being ignored, and that `cursor` is ABSENT rather than inert
// (DEC-IYHLNF): a script written against stage-2 paging must fail loudly here,
// not silently re-read page one forever.
func TestListEntities_RejectsBadOptions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, call, wantMsg string }{
		{"cursor not yet supported", `{cursor = "abc"}`, "unknown option"},
		{"typo'd key", `{limt = 5}`, "unknown option"},
		{"limit zero", `{limit = 0}`, "must be positive"},
		{"limit negative", `{limit = -5}`, "must be positive"},
		{"limit non-number", `{limit = true}`, "must be a number"},
		{"filter non-string", `{filter = 42}`, "must be a string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runList(t, &countingReader{n: 5},
				`rela.list_entities("ticket", `+tc.call+`)`)
			if err == nil {
				t.Fatalf("%s was accepted silently", tc.call)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %v, want it to mention %q", err, tc.wantMsg)
			}
		})
	}
}

// swapMaxReadLimit lowers the ceiling for one test and returns a restore func.
// The ceiling is a var precisely so tests need not seed thousands of rows.
//
// Callers must NOT be parallel: this mutates package state, and a parallel
// test (or a parallel subtest, which resumes after its parent returns) would
// observe the restored value instead. Setup verifies the swap took effect, so
// that mistake fails here rather than as a confusing assertion miss later.
func swapMaxReadLimit(t *testing.T, n int) func() {
	t.Helper()
	prev := maxReadLimit
	maxReadLimit = n
	if maxReadLimit != n {
		t.Fatalf("ceiling swap did not take: got %d, want %d", maxReadLimit, n)
	}
	return func() { maxReadLimit = prev }
}

var errStubRead = errors.New("stub reader: synthetic failure")
