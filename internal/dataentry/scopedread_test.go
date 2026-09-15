package dataentry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestScopedHeaders_NarrowingsApplyToBothVerdictBranches is the regression
// this whole extraction exists to make impossible.
//
// RR-GQWRLD (world) and TKT-O7R2A1 (faces) were both the same defect: a
// narrowing stamped on the ACL-gated branch but not the AllowAll one, so the
// most privileged population silently read wider than everyone else. While
// writing scopedHeaders I reintroduced it a THIRD time, for Props — caught
// only incidentally by a next-action test.
//
// So this asserts the invariant per dimension, directly: for each narrowing,
// an AllowAll principal and an ACL-gated principal must see the same
// narrowing applied.
func TestScopedHeaders_NarrowingsApplyToBothVerdictBranches(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-mine", Type: "ticket",
		Properties: map[string]any{"title": "mine", "status": "open"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-theirs", Type: "ticket",
		Properties: map[string]any{"title": "theirs", "status": "closed"},
	})

	// An ACL-gated verdict that permits the whole type: it differs from
	// AllowAll only in WHICH BRANCH it takes, so any difference in the
	// result is the branch skew this test hunts.
	gated := acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket"}}
	allowAll := acl.ReadQueryResult{AllowAll: true}

	tests := []struct {
		name string
		req  scopeRequest
		want []string
	}{
		{
			name: "no narrowing returns everything",
			req:  scopeRequest{Type: "ticket"},
			want: []string{"TKT-mine", "TKT-theirs"},
		},
		{
			name: "Props narrows (the bug this extraction introduced)",
			req: scopeRequest{Type: "ticket", Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
			}},
			want: []string{"TKT-mine"},
		},
		{
			name: "Props that match nothing return nothing on both branches",
			req: scopeRequest{Type: "ticket", Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "nonesuch", Scalar: true},
			}},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			gotAllowAll := idsOf(t, ctx, app, allowAll, tc.req)
			gotGated := idsOf(t, ctx, app, gated, tc.req)

			if strings.Join(gotAllowAll, ",") != strings.Join(gotGated, ",") {
				t.Errorf("verdict branches disagree: AllowAll=%v, ACL-gated=%v — "+
					"a narrowing carried on one branch only is the RR-GQWRLD fail-open",
					gotAllowAll, gotGated)
			}
			if got := strings.Join(gotAllowAll, ","); got != strings.Join(tc.want, ",") {
				t.Errorf("ids = %v, want %v", gotAllowAll, tc.want)
			}
		})
	}
}

func idsOf(
	t *testing.T, ctx context.Context, app *App, rqr acl.ReadQueryResult, req scopeRequest,
) []string {
	t.Helper()
	headers, _, err := scopedHeaders(ctx, app.Services(), rqr, req)
	if err != nil {
		t.Fatalf("scopedHeaders: %v", err)
	}
	var ids []string
	for _, h := range headers {
		ids = append(ids, h.ID)
	}
	sortStrings(ids)
	return ids
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// TestScopedHeaders_ZeroVerdictIsRefused pins the defensive branch: a zero
// ReadQueryResult has AllowAll=false, DenyAll=false and Query=nil, so a
// switch that forgot the case would fall through to the ACL branch and
// dereference nil — or, worse, an earlier arrangement would alias AllowAll
// and disclose the whole type.
func TestScopedHeaders_ZeroVerdictIsRefused(t *testing.T) {
	app := newTestAppV1(t)
	_, _, err := scopedHeaders(context.Background(), app.Services(),
		acl.ReadQueryResult{}, scopeRequest{Type: "ticket"})
	if err == nil {
		t.Fatal("a zero ReadQueryResult was accepted; it must be refused, never treated as AllowAll")
	}
	if !strings.Contains(err.Error(), "zero ReadQueryResult") {
		t.Errorf("error = %v, want it to name the zero verdict", err)
	}
}

// TestScopedHeaders_DenyAllReportsWithheld pins the withheld/empty split. The
// two are identical on the wire, but a caller must be able to tell them apart
// to skip downstream work — running a free-text search for a principal who may
// read nothing lets them probe backend latency through ?q= (RR-X56H).
func TestScopedHeaders_DenyAllReportsWithheld(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket"})

	_, withheld, err := scopedHeaders(context.Background(), app.Services(),
		acl.ReadQueryResult{DenyAll: true}, scopeRequest{Type: "ticket"})
	if err != nil {
		t.Fatalf("scopedHeaders: %v", err)
	}
	if !withheld {
		t.Error("DenyAll did not report withheld; the caller cannot short-circuit")
	}

	// A permitted read that matches nothing is NOT withheld.
	_, withheld, err = scopedHeaders(context.Background(), app.Services(),
		acl.ReadQueryResult{AllowAll: true}, scopeRequest{
			Type: "ticket",
			Props: []store.PropPredicate{
				{Property: "status", Op: store.PropEqual, Value: "nonesuch", Scalar: true},
			},
		})
	if err != nil {
		t.Fatalf("scopedHeaders: %v", err)
	}
	if withheld {
		t.Error("an empty permitted read reported withheld; that conflates 'nothing matched' with 'refused'")
	}
}

// TestScopedHeaders_DoesNotMutateTheACLQuery pins the copy. The ACL layer may
// cache a ReadQueryResult per principal, so stamping the shared *GraphQuery in
// place would leak one request's narrowing into the next caller's.
func TestScopedHeaders_DoesNotMutateTheACLQuery(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"status": "open"}})

	shared := &store.GraphQuery{EntityType: "ticket"}
	rqr := acl.ReadQueryResult{Query: shared}

	_, _, err := scopedHeaders(context.Background(), app.Services(), rqr, scopeRequest{
		Type:  "ticket",
		Props: []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
	})
	if err != nil {
		t.Fatalf("scopedHeaders: %v", err)
	}
	if len(shared.Props) != 0 {
		t.Errorf("the shared ACL GraphQuery gained %d Props; a cached verdict would "+
			"carry this request's narrowing into the next principal's read", len(shared.Props))
	}
}

// verdictSwitch matches the shape scopedHeaders owns: a branch on the ACL
// read verdict's AllowAll field. A second one in this package means a
// collection read that will not inherit the next narrowing dimension.
var verdictSwitch = regexp.MustCompile(`(?m)^\s*case\s+\w+\.AllowAll\b`)

// verdictSwitchExempt lists files allowed to branch on AllowAll.
//
// An EXEMPTION list, not an inclusion list, so a new file fails closed: it
// must either route through scopedHeaders or be named here with a reason.
// This mirrors internal/acl/ceilingguard_test.go, and exists for the same
// reason — the convention it enforces ("stamp every narrowing on every
// branch") was previously guarded only by a comment asking reviewers to grep,
// and two narrowings slipped through anyway.
var verdictSwitchExempt = map[string]string{
	"scopedread.go": "the funnel itself — this is the one verdict switch",

	// Translates a verdict into a search.TypeScope; reads nothing. The scope
	// it emits is consumed by visibleEntitiesOfType, which DOES route through
	// the funnel, so a narrowing added there still reaches search.
	"readgate.go": "verdict -> search.TypeScope adapter, not a read",

	// Builds a paged store.GraphQuery for the pushdown fast path (ordering,
	// limit, offset) rather than materialising a slice, so it cannot return
	// the funnel's headers. It is the ONE duplicate that must stay, and it is
	// the one place to check when adding a narrowing: planListPushdown and
	// scopedHeaders serve the same endpoint by different routes, so a
	// dimension added to one and not the other makes a list's first page
	// disagree with its filtered page.
	"listpushdown.go": "paged query PLAN, not a materialised read — see the note in scopedread.go",

	// ganttReadVerdict classifies the verdict to choose a REDACTION path
	// (header-only fast path vs full entities), not to build a query. Both
	// of its arms read through the funnel: the AllowAll arm via
	// h.scopedHeaders, the scoped arm via h.scoped (= scopedSortedEntities).
	"gantt_handler.go": "verdict -> redaction-path classifier; both arms read through the funnel",
}

func TestScopedHeaders_IsTheOnlyVerdictSwitch(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := verdictSwitchExempt[name]; ok {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if loc := verdictSwitch.FindIndex(src); loc != nil {
			line := 1 + strings.Count(string(src[:loc[0]]), "\n")
			t.Errorf("%s:%d branches on an ACL read verdict directly.\n"+
				"Collection reads must route through scopedHeaders (scopedread.go) so a new "+
				"narrowing reaches every surface — see RR-GQWRLD and TKT-O7R2A1 for what "+
				"happens otherwise. If this genuinely is not a collection read, add it to "+
				"verdictSwitchExempt with a reason.", name, line)
		}
	}
}

// TestScopedHeaders_ScopeFiltersOnBothBranches extends the branch-parity
// invariant to the query-scope narrowing. Same reasoning as Props: a scope
// applied on one verdict branch only would show an AllowAll principal the
// rows everyone else's scope hides.
func TestScopedHeaders_ScopeFiltersOnBothBranches(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-open", Type: "ticket",
		Properties: map[string]any{"status": "open"}})
	seedEntity(app, &entity.Entity{ID: "TKT-done", Type: "ticket",
		Properties: map[string]any{"status": "done"}})

	// A stand-in scope: "status is not done". The real one is a compiled
	// predicate program, but the funnel only ever calls the evaluator, so a
	// closure exercises the same path.
	notDone := scopeRequest{
		Type:  "ticket",
		Scope: "not-done",
		ScopeEval: func(_ context.Context, _ QueryScopeHandle, _, _ string, props map[string]any) (bool, error) {
			return props["status"] != "done", nil
		},
	}

	gated := acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket"}}
	allowAll := acl.ReadQueryResult{AllowAll: true}

	gotAllowAll := idsOf(t, context.Background(), app, allowAll, notDone)
	gotGated := idsOf(t, context.Background(), app, gated, notDone)

	if strings.Join(gotAllowAll, ",") != strings.Join(gotGated, ",") {
		t.Errorf("verdict branches disagree under a scope: AllowAll=%v, ACL-gated=%v",
			gotAllowAll, gotGated)
	}
	if got := strings.Join(gotAllowAll, ","); got != "TKT-open" {
		t.Errorf("scoped ids = %v, want [TKT-open]", gotAllowAll)
	}
}

// TestScopedHeaders_ScopeWithoutEvaluatorIsRefused pins the fail-closed
// direction. A nil evaluator beside a non-nil scope must error: silently
// skipping the filter would serve the UNSCOPED set, which is exactly what a
// scope exists to prevent, and nothing on screen would say so.
func TestScopedHeaders_ScopeWithoutEvaluatorIsRefused(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket"})

	_, _, err := scopedHeaders(context.Background(), app.Services(),
		acl.ReadQueryResult{AllowAll: true},
		scopeRequest{Type: "ticket", Scope: "something", ScopeEval: nil})
	if err == nil {
		t.Fatal("a scope with no evaluator was accepted; it must refuse rather than serve unscoped rows")
	}
}

// TestScopedHeaders_ScopeErrorPropagates pins that an evaluation failure
// fails the request. Folding it into a non-match would render an empty page
// for an identity scope with no principal — indistinguishable from "you have
// no tasks", which is a wrong answer presented as a right one.
func TestScopedHeaders_ScopeErrorPropagates(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket"})

	_, _, err := scopedHeaders(context.Background(), app.Services(),
		acl.ReadQueryResult{AllowAll: true}, scopeRequest{
			Type:  "ticket",
			Scope: "identity",
			ScopeEval: func(_ context.Context, _ QueryScopeHandle, _, _ string, _ map[string]any) (bool, error) {
				return false, errNoIdentityForTest
			},
		})
	if err == nil {
		t.Fatal("a scope evaluation error was swallowed; it must fail the request")
	}
	if !errors.Is(err, errNoIdentityForTest) {
		t.Errorf("error = %v, want it to wrap the evaluator's error", err)
	}
}

var errNoIdentityForTest = errors.New("no current user")

// TestScopedHeaders_ScopePropsAreASuperset pins the pushdown contract: the
// pushed conjuncts narrow the READ, and the Go-side scope remains
// authoritative. Passing props that are broader than the scope must not
// change the result — only how many rows the store returned.
func TestScopedHeaders_ScopePropsAreASuperset(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-a", Type: "ticket",
		Properties: map[string]any{"status": "open", "prio": "high"}})
	seedEntity(app, &entity.Entity{ID: "TKT-b", Type: "ticket",
		Properties: map[string]any{"status": "open", "prio": "low"}})
	seedEntity(app, &entity.Entity{ID: "TKT-c", Type: "ticket",
		Properties: map[string]any{"status": "done", "prio": "high"}})

	// The scope is "open AND high". Only the first conjunct is pushable.
	req := scopeRequest{
		Type:       "ticket",
		ScopeProps: []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
		Scope:      "open-and-high",
		ScopeEval: func(_ context.Context, _ QueryScopeHandle, _, _ string, props map[string]any) (bool, error) {
			return props["status"] == "open" && props["prio"] == "high", nil
		},
	}
	got := idsOf(t, context.Background(), app, acl.ReadQueryResult{AllowAll: true}, req)
	if strings.Join(got, ",") != "TKT-a" {
		t.Errorf("ids = %v, want [TKT-a]", got)
	}

	// The SAME scope with no pushdown must give the same answer, slower.
	req.ScopeProps = nil
	if unpushed := idsOf(t, context.Background(), app, acl.ReadQueryResult{AllowAll: true}, req); strings.Join(unpushed, ",") != strings.Join(got, ",") {
		t.Errorf("pushed=%v unpushed=%v — the prefilter changed the answer, so it is not a superset",
			got, unpushed)
	}
}
