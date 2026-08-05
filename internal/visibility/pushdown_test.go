package visibility

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// countingRedactor records how many rows it redacted and strips "secret".
type countingRedactor struct{ calls int }

func (c *countingRedactor) redact(_ context.Context, e *entity.Entity) *entity.Entity {
	c.calls++
	if e == nil || e.Properties["secret"] == nil {
		return e
	}
	out := *e
	out.Properties = map[string]any{}
	for k, v := range e.Properties {
		if k != "secret" {
			out.Properties[k] = v
		}
	}
	return &out
}

// stubProvider returns a fixed ReadQueryResult.
type stubProvider struct {
	res acl.ReadQueryResult
	err error
}

func (s stubProvider) ReadQueryFor(context.Context, string) (acl.ReadQueryResult, error) {
	return s.res, s.err
}

// graphSpy is a store.Store stand-in that records whether GraphQuery ran.
type graphSpy struct {
	store.Store
	graphCalls int
	listCalls  int
	rows       []*entity.Entity
}

func (g *graphSpy) GraphQuery(context.Context, store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	g.graphCalls++
	return g.seq()
}

func (g *graphSpy) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	g.listCalls++
	return g.seq()
}

func (g *graphSpy) seq() iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range g.rows {
			if !yield(e, nil) {
				return
			}
		}
	}
}

// GraphCount and MatchingIDs complete store.GraphQueryer. Neither is on the
// pushdown path (it uses GraphQuery only), so they are inert here — a
// non-zero return would misrepresent them as participating.
func (g *graphSpy) GraphCount(context.Context, store.GraphQuery) (matched, total int, err error) {
	return 0, 0, nil
}

func (g *graphSpy) MatchingIDs(
	context.Context, store.GraphQuery, []string,
) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func seededSpy() *graphSpy {
	return &graphSpy{rows: []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "A", "secret": "S"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "B"}},
	}}
}

func drain(t *testing.T, seq iter.Seq2[*entity.Entity, error]) ([]*entity.Entity, error) {
	t.Helper()
	var out []*entity.Entity
	for e, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, e)
	}
	return out, nil
}

// TestListPushdown_QueryBranchUsesGraphQuery pins that a composed ACL scope
// actually reaches the store as a query. Without this, every other test here
// could pass while the pushdown silently fell back to load-then-Filter.
func TestListPushdown_QueryBranchUsesGraphQuery(t *testing.T) {
	t.Parallel()
	spy := seededSpy()
	red := &countingRedactor{}
	p := stubProvider{res: acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket"}}}

	seq, ok := listPushdown(context.Background(), p, spy, red.redact, store.EntityQuery{Type: "ticket"})
	if !ok {
		t.Fatal("pushdown declined a composable query")
	}
	rows, err := drain(t, seq)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if spy.graphCalls != 1 {
		t.Errorf("GraphQuery calls = %d, want 1 — the ACL predicate never reached the store", spy.graphCalls)
	}
	if spy.listCalls != 0 {
		t.Errorf("ListEntities calls = %d, want 0 — pushdown must not also do a full scan", spy.listCalls)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
}

// TestListPushdown_RedactsOnEveryBranch is the RR-1W1G6K regression test.
// The pushdown replaces the ROW gate; dropping field redaction with it would
// return every `visible:`-hidden property to scripts — the #1188 finding.
//
// AllowAll is called out separately (RR-OXE47R): "may read every row" is not
// "may see every field", and it is the branch where a return-rows-straight-
// through shortcut is most tempting.
func TestListPushdown_RedactsOnEveryBranch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		res  acl.ReadQueryResult
	}{
		{"AllowAll", acl.ReadQueryResult{AllowAll: true}},
		{"Query", acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spy := seededSpy()
			red := &countingRedactor{}
			seq, ok := listPushdown(
				context.Background(), stubProvider{res: tc.res}, spy, red.redact,
				store.EntityQuery{Type: "ticket"})
			if !ok {
				t.Fatal("pushdown declined")
			}
			rows, err := drain(t, seq)
			if err != nil {
				t.Fatalf("drain: %v", err)
			}
			if red.calls != len(rows) {
				t.Errorf("redactor ran %d times for %d rows — every row must be redacted",
					red.calls, len(rows))
			}
			for _, e := range rows {
				if _, leaked := e.Properties["secret"]; leaked {
					t.Errorf("row %s leaked a hidden property: %v", e.ID, e.Properties)
				}
			}
		})
	}
}

// TestListPushdown_DenyAllYieldsNothingWithoutTouchingTheStore pins that a
// denied scope short-circuits rather than reading and filtering.
func TestListPushdown_DenyAllYieldsNothingWithoutTouchingTheStore(t *testing.T) {
	t.Parallel()
	spy := seededSpy()
	red := &countingRedactor{}
	seq, ok := listPushdown(
		context.Background(), stubProvider{res: acl.ReadQueryResult{DenyAll: true}},
		spy, red.redact, store.EntityQuery{Type: "ticket"})
	if !ok {
		t.Fatal("pushdown declined DenyAll")
	}
	rows, err := drain(t, seq)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("DenyAll yielded %d rows, want 0", len(rows))
	}
	if spy.graphCalls+spy.listCalls != 0 {
		t.Errorf("DenyAll touched the store (%d graph, %d list), want 0",
			spy.graphCalls, spy.listCalls)
	}
}

// TestListPushdown_ScopeErrorFailsClosed pins that a scope we cannot compose
// surfaces as an error rather than degrading to an ungated read. An empty
// list would be the other tempting choice and is wrong: it is
// indistinguishable from "you may see nothing" (the TKT-FVQ4 ambiguity).
func TestListPushdown_ScopeErrorFailsClosed(t *testing.T) {
	t.Parallel()
	spy := seededSpy()
	red := &countingRedactor{}
	boom := errors.New("gate down")
	seq, ok := listPushdown(
		context.Background(), stubProvider{err: boom}, spy, red.redact,
		store.EntityQuery{Type: "ticket"})
	if !ok {
		t.Fatal("a scope error must be reported, not silently declined to the fallback")
	}
	rows, err := drain(t, seq)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the gate failure surfaced", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows leaked past a failed scope: %v", rows)
	}
	if spy.graphCalls+spy.listCalls != 0 {
		t.Errorf("store was read despite an uncomposable scope")
	}
}

// TestListPushdown_DeclinesWhenNotApplicable pins the fallback conditions.
// Declining is a PERFORMANCE regression only — the caller then runs
// load-then-Filter, which gates on the same policy.
func TestListPushdown_DeclinesWhenNotApplicable(t *testing.T) {
	t.Parallel()
	red := &countingRedactor{}
	q := store.EntityQuery{Type: "ticket"}
	allow := stubProvider{res: acl.ReadQueryResult{AllowAll: true}}

	if _, ok := listPushdown(context.Background(), nil, seededSpy(), red.redact, q); ok {
		t.Error("pushdown ran without a provider")
	}
	if _, ok := listPushdown(
		context.Background(), allow, seededSpy(), red.redact, store.EntityQuery{},
	); ok {
		t.Error("pushdown ran for a type-less query; the ACL scope is composed per type")
	}
	// Zero ReadQueryResult — neither allow, deny, nor query. Unrepresentable;
	// must fall back rather than guess.
	if _, ok := listPushdown(
		context.Background(), stubProvider{}, seededSpy(), red.redact, q,
	); ok {
		t.Error("pushdown accepted a zero ReadQueryResult instead of falling back")
	}
}
