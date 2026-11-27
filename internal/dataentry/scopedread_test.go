package dataentry

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
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
	gated := acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket", Faces: store.InWorld(store.TrivialScope())}}
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
		{
			name: "ByIDs narrows to the listed ids",
			req:  scopeRequest{Type: "ticket", ByIDs: true, IDs: []string{"TKT-mine"}},
			want: []string{"TKT-mine"},
		},
		{
			name: "ByIDs with no ids reads nothing",
			req:  scopeRequest{Type: "ticket", ByIDs: true},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			gotAllowAll := idsOf(ctx, t, app, allowAll, tc.req)
			gotGated := idsOf(ctx, t, app, gated, tc.req)

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
	ctx context.Context, t *testing.T, app *App, rqr acl.ReadQueryResult, req scopeRequest,
) []string {
	t.Helper()
	headers, _, err := readScoped(ctx, app.Services(), rqr, req)
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

// readScoped reads req the way production code does: a bounded request
// through scopedHeaders, every row through scopedQuery and scanHeaders.
func readScoped(
	ctx context.Context, svc Services, rqr acl.ReadQueryResult, req scopeRequest,
) ([]store.EntityHeader, bool, error) {
	if req.ByIDs || req.Limit > 0 || req.Offset > 0 {
		return scopedHeaders(ctx, svc, rqr, req)
	}
	q, errClass, withheld, err := scopedQuery(ctx, rqr, req)
	if withheld || err != nil {
		return nil, withheld, err
	}
	headers, err := scanHeaders(ctx, svc, q, errClass, req)
	return headers, false, err
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
	_, _, err := readScoped(context.Background(), app.Services(),
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

	_, withheld, err := readScoped(context.Background(), app.Services(),
		acl.ReadQueryResult{DenyAll: true}, scopeRequest{Type: "ticket"})
	if err != nil {
		t.Fatalf("scopedHeaders: %v", err)
	}
	if !withheld {
		t.Error("DenyAll did not report withheld; the caller cannot short-circuit")
	}

	// A permitted read that matches nothing is NOT withheld.
	_, withheld, err = readScoped(context.Background(), app.Services(),
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

	shared := &store.GraphQuery{EntityType: "ticket", Faces: store.InWorld(store.TrivialScope())}
	rqr := acl.ReadQueryResult{Query: shared}

	_, _, err := readScoped(context.Background(), app.Services(), rqr, scopeRequest{
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
	// limit, offset) rather than materializing a slice, so it cannot return
	// the funnel's headers. It is the ONE duplicate that must stay, and it is
	// the one place to check when adding a narrowing: planListPushdown and
	// scopedHeaders serve the same endpoint by different routes, so a
	// dimension added to one and not the other makes a list's first page
	// disagree with its filtered page.
	"listpushdown.go": "paged query PLAN, not a materialized read — see the note in scopedread.go",

	// ganttReadVerdict classifies the verdict to choose a REDACTION path
	// (header-only fast path vs full entities), not to build a query. Both
	// of its arms read through the funnel: the AllowAll arm via
	// h.scanTypeHeaders, the scoped arm via h.scoped (= scopedSortedEntities).
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
		ScopeFilter: rowFilter(func(props map[string]any) (bool, error) {
			return props["status"] != "done", nil
		}),
	}

	gated := acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "ticket", Faces: store.InWorld(store.TrivialScope())}}
	allowAll := acl.ReadQueryResult{AllowAll: true}

	gotAllowAll := idsOf(context.Background(), t, app, allowAll, notDone)
	gotGated := idsOf(context.Background(), t, app, gated, notDone)

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

	_, _, err := readScoped(context.Background(), app.Services(),
		acl.ReadQueryResult{AllowAll: true},
		scopeRequest{Type: "ticket", Scope: "something", ScopeFilter: nil})
	if err == nil {
		t.Fatal("a scope with no evaluator was accepted; it must refuse rather than serve unscoped rows")
	}
}

// TestScopedHeaders_WindowWithScopeIsRefused pins that a read window is not
// combined with a scope. The scope filters after the store read, so a window
// of the store's rows would not be a window of the result (TKT-CKLZMWB).
func TestScopedHeaders_WindowWithScopeIsRefused(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket"})

	for _, req := range []scopeRequest{
		{Type: "ticket", Scope: "something", Limit: 20},
		{Type: "ticket", Scope: "something", Offset: 20},
	} {
		_, _, err := scopedHeaders(context.Background(), app.Services(),
			acl.ReadQueryResult{AllowAll: true}, req)
		if !errors.Is(err, errListLoad) || !strings.Contains(err.Error(), "read window") {
			t.Fatalf("limit %d offset %d: err = %v, want the read-window refusal", req.Limit, req.Offset, err)
		}
	}
}

// TestScopedHeaders_ScopeErrorPropagates pins that an evaluation failure
// fails the request. Folding it into a non-match would render an empty page
// for an identity scope with no principal — indistinguishable from "you have
// no tasks", which is a wrong answer presented as a right one.
func TestScopedHeaders_ScopeErrorPropagates(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket"})

	_, _, err := readScoped(context.Background(), app.Services(),
		acl.ReadQueryResult{AllowAll: true}, scopeRequest{
			Type:  "ticket",
			Scope: "identity",
			ScopeFilter: rowFilter(func(map[string]any) (bool, error) {
				return false, errNoIdentityForTest
			}),
		})
	if err == nil {
		t.Fatal("a scope evaluation error was swallowed; it must fail the request")
	}
	if !errors.Is(err, errNoIdentityForTest) {
		t.Errorf("error = %v, want it to wrap the evaluator's error", err)
	}
}

var errNoIdentityForTest = errors.New("no current user")

// rowFilter adapts a per-row stand-in scope to [QueryScopeFilter]. The real
// filter is a compiled predicate program; the funnel only ever calls the
// filter, so a closure exercises the same path.
func rowFilter(keep func(props map[string]any) (bool, error)) QueryScopeFilter {
	return func(
		_ context.Context, _ QueryScopeHandle, _ string, headers []store.EntityHeader,
		_ func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
		_ func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
	) ([]store.EntityHeader, error) {
		var out []store.EntityHeader
		for _, h := range headers {
			ok, err := keep(h.Properties)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, h)
			}
		}
		return out, nil
	}
}

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
		ScopeFilter: rowFilter(func(props map[string]any) (bool, error) {
			return props["status"] == "open" && props["prio"] == "high", nil
		}),
	}
	got := idsOf(context.Background(), t, app, acl.ReadQueryResult{AllowAll: true}, req)
	if strings.Join(got, ",") != "TKT-a" {
		t.Errorf("ids = %v, want [TKT-a]", got)
	}

	// The SAME scope with no pushdown must give the same answer, slower.
	req.ScopeProps = nil
	if unpushed := idsOf(context.Background(), t, app, acl.ReadQueryResult{AllowAll: true}, req); strings.Join(unpushed, ",") != strings.Join(got, ",") {
		t.Errorf("pushed=%v unpushed=%v — the prefilter changed the answer, so it is not a superset",
			got, unpushed)
	}
}

// TestScopedHeaders_ByIDsIsTheFullReadRestricted pins the id narrowing
// (TKT-F2EJQY8): reading by ids returns exactly the rows of the full read
// whose id is listed, on both verdict branches, so the ids can only narrow.
// ByIDs with no ids reads nothing; reading it as "no narrowing" would serve
// the whole type.
func TestScopedHeaders_ByIDsIsTheFullReadRestricted(t *testing.T) {
	app := newTestAppV1(t)
	for _, e := range []*entity.Entity{
		{ID: "TKT-a", Type: "ticket", Properties: map[string]any{"status": "open"}},
		{ID: "TKT-b", Type: "ticket", Properties: map[string]any{"status": "closed"}},
		{ID: "TKT-c", Type: "ticket", Properties: map[string]any{"status": "open"}},
		{ID: "F-1", Type: "feature", Properties: map[string]any{"status": "open"}},
	} {
		seedEntity(app, e)
	}
	openOnly := acl.ReadQueryResult{Query: &store.GraphQuery{
		EntityType: "ticket",
		Props:      []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
	}}
	allowAll := acl.ReadQueryResult{AllowAll: true}
	closedProp := []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "closed", Scalar: true}}

	tests := []struct {
		name string
		rqr  acl.ReadQueryResult
		req  scopeRequest
		want []string
	}{
		{"allow-all", allowAll, scopeRequest{Type: "ticket", ByIDs: true, IDs: []string{"TKT-c", "TKT-b"}},
			[]string{"TKT-b", "TKT-c"}},
		{"verdict drops a listed id", openOnly,
			scopeRequest{Type: "ticket", ByIDs: true, IDs: []string{"TKT-a", "TKT-b"}}, []string{"TKT-a"}},
		{"props drop a listed id", allowAll,
			scopeRequest{Type: "ticket", Props: closedProp, ByIDs: true, IDs: []string{"TKT-a", "TKT-b"}},
			[]string{"TKT-b"}},
		{"another type's id is not read", allowAll,
			scopeRequest{Type: "ticket", ByIDs: true, IDs: []string{"F-1", "TKT-a"}}, []string{"TKT-a"}},
		{"unknown id", allowAll, scopeRequest{Type: "ticket", ByIDs: true, IDs: []string{"TKT-nope"}}, nil},
		{"no ids reads nothing", allowAll, scopeRequest{Type: "ticket", ByIDs: true}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			got := idsOf(ctx, t, app, tc.rqr, tc.req)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("ids = %v, want %v", got, tc.want)
			}
			// The same rows as the full read restricted to the ids.
			full := tc.req
			full.ByIDs, full.IDs = false, nil
			listed := make(map[string]bool, len(tc.req.IDs))
			for _, id := range tc.req.IDs {
				listed[id] = true
			}
			var want []string
			for _, id := range idsOf(ctx, t, app, tc.rqr, full) {
				if listed[id] {
					want = append(want, id)
				}
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("by ids = %v, full read restricted = %v", got, want)
			}
		})
	}
}

// TestScopedHeaders_ByIDsKeepsTheWorldsFace pins the id read on a faced
// type: the world picks each id's face exactly as the full read does, and a
// face allowlist that excludes the first face in the chain falls through to
// the next one rather than dropping the id.
func TestScopedHeaders_ByIDsKeepsTheWorldsFace(t *testing.T) {
	app, _ := facedApp(t, nil)
	ctx := context.Background()
	// facedApp seeds POL-1 at draft and published; POL-2 exists as a draft.
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "POL-2", Type: "policy", Face: "draft", Properties: map[string]any{"title": "d"},
	}); err != nil {
		t.Fatal(err)
	}
	ctx = withWorld(ctx, worldHandle{name: "published", scope: rankedPolicyWorld()})
	gated := acl.ReadQueryResult{Query: &store.GraphQuery{EntityType: "policy"}}
	allowAll := acl.ReadQueryResult{AllowAll: true}

	faceRefs := func(t *testing.T, rqr acl.ReadQueryResult, req scopeRequest) string {
		t.Helper()
		headers, _, err := readScoped(ctx, app.Services(), rqr, req)
		if err != nil {
			t.Fatalf("scopedHeaders: %v", err)
		}
		refs := make([]string, 0, len(headers))
		for _, h := range headers {
			refs = append(refs, h.ID+"@"+h.Face.String())
		}
		return strings.Join(refs, ",")
	}
	for _, tc := range []struct {
		name  string
		faces []entity.Face
		ids   []string
		want  string
	}{
		{"the world's first face", nil, []string{"POL-1", "POL-2"}, "POL-1@published,POL-2@draft"},
		{"one id of two", nil, []string{"POL-1"}, "POL-1@published"},
		{"an allowlist falls through the chain", []entity.Face{"draft"}, []string{"POL-1"}, "POL-1@draft"},
	} {
		for name, rqr := range map[string]acl.ReadQueryResult{"allow-all": allowAll, "gated": gated} {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				req := scopeRequest{Type: "policy", Faces: tc.faces, ByIDs: true, IDs: tc.ids}
				if got := faceRefs(t, rqr, req); got != tc.want {
					t.Fatalf("by ids = %s, want %s", got, tc.want)
				}
				full := scopeRequest{Type: "policy", Faces: tc.faces}
				var restricted []string
				for ref := range strings.SplitSeq(faceRefs(t, rqr, full), ",") {
					for _, id := range tc.ids {
						if strings.HasPrefix(ref, id+"@") {
							restricted = append(restricted, ref)
						}
					}
				}
				if got, want := faceRefs(t, rqr, req), strings.Join(restricted, ","); got != want {
					t.Fatalf("by ids = %s, full read restricted = %s", got, want)
				}
			})
		}
	}
}

// TestScopedHeaders_UnboundedReadIsRefused pins that the bounded read stays
// bounded: a request with no ids and no window is an error, so a caller that
// needs every row has to say so through scanHeaders (TKT-C5H506O).
func TestScopedHeaders_UnboundedReadIsRefused(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket"})

	for _, req := range []scopeRequest{
		{Type: "ticket"},
		{Type: "ticket", Offset: 5},
		{Type: "ticket", Limit: -1},
		{Type: "ticket", Limit: 5, Offset: -1},
	} {
		_, _, err := scopedHeaders(context.Background(), app.Services(), acl.ReadQueryResult{AllowAll: true}, req)
		if !errors.Is(err, errListLoad) {
			t.Errorf("limit %d offset %d: err = %v, want a refusal", req.Limit, req.Offset, err)
		}
	}
}

// TestScopedHeaders_WindowReadsOneWindow pins the window branch: rows in id
// order, cut after the row gate.
func TestScopedHeaders_WindowReadsOneWindow(t *testing.T) {
	app := newTestAppV1(t)
	for _, id := range []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"status": "open"}})
	}
	seedEntity(app, &entity.Entity{ID: "TKT-0", Type: "ticket", Properties: map[string]any{"status": "hidden"}})
	open := []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}}
	gated := acl.ReadQueryResult{Query: &store.GraphQuery{
		EntityType: "ticket", Props: open, Faces: store.InWorld(store.TrivialScope()),
	}}
	got := idsOf(context.Background(), t, app, gated, scopeRequest{Type: "ticket", Limit: 2, Offset: 1})
	if strings.Join(got, ",") != "TKT-2,TKT-3" {
		t.Errorf("window = %v, want [TKT-2 TKT-3]: the hidden TKT-0 must not take a slot", got)
	}
}

// TestScanHeaders_RefusesAQueryForAnotherType pins the guard against a caller
// that skipped the withheld check: the zero query scopedQuery returns then has
// no type, and several backends read every type for it.
func TestScanHeaders_RefusesAQueryForAnotherType(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket"})

	req := scopeRequest{Type: "ticket"}
	for _, q := range []store.GraphQuery{{}, {EntityType: "person"}} {
		headers, err := scanHeaders(context.Background(), app.Services(), q, errListLoad, req)
		if !errors.Is(err, errListLoad) || len(headers) != 0 {
			t.Errorf("query for %q: headers %d, err = %v; want a refusal", q.EntityType, len(headers), err)
		}
	}
}

// TestScanHeaders_TakesItsQueryFromScopedQuery pins the call shape scanHeaders
// relies on. The query carries the verdict, world and faces; the request
// carries the query scope, which scanHeaders applies. A request other than
// the one the query was built from would skip or change the scope, which is
// the RR-GQWRLD fail-open shape. So, in every function that calls scanHeaders:
//
//   - the query, error class and request are plain variables named by one
//     scopedQuery assignment in that function;
//   - the query is assigned once, and changed after that only in its Offset
//     or Limit;
//   - the request is assigned at most once and never changed field by field.
//
// Not caught: a variable shadowed in an inner block, since names are compared
// rather than objects. Review covers that.
func TestScanHeaders_TakesItsQueryFromScopedQuery(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			var body *ast.BlockStmt
			switch fn := n.(type) {
			case *ast.FuncDecl:
				body = fn.Body
			case *ast.FuncLit:
				body = fn.Body
			}
			if body == nil {
				return true
			}
			for _, call := range directCalls(body, "scanHeaders") {
				calls++
				if msg := scanCallProblem(body, call); msg != "" {
					t.Errorf("%s: %s", fset.Position(call.Pos()), msg)
				}
			}
			return true
		})
	}
	if calls == 0 {
		t.Fatal("found no scanHeaders call; the guard matches nothing")
	}
}

// scanCallProblem says what is wrong with one scanHeaders call in body, or
// returns "" when it has the shape TestScanHeaders_TakesItsQueryFromScopedQuery
// requires.
func scanCallProblem(body *ast.BlockStmt, call *ast.CallExpr) string {
	if len(call.Args) != 5 {
		return "scanHeaders with the wrong number of arguments"
	}
	q, class, req := identName(call.Args[2]), identName(call.Args[3]), identName(call.Args[4])
	if q == "" || class == "" || req == "" {
		return "scanHeaders takes its query, error class and request as plain variables"
	}
	var built []scopedQueryAssignment
	for _, a := range scopedQueryAssignments(body) {
		if a.query == q {
			built = append(built, a)
		}
	}
	switch {
	case len(built) != 1:
		return "the query " + q + " is not assigned by exactly one scopedQuery call in this function"
	case built[0].class != class || built[0].req != req:
		return "scanHeaders(" + q + ", " + class + ", " + req + ") does not take the error class and request " +
			"of the scopedQuery call that built " + q
	}
	qWrites := writesTo(body, q)
	for _, w := range qWrites {
		if w != "" && w != "Offset" && w != "Limit" {
			return "the query " + q + " is changed after scopedQuery (" + w + "); only Offset and Limit may be"
		}
	}
	if countEmpty(qWrites) != 1 {
		return "the query " + q + " is reassigned after scopedQuery"
	}
	reqWrites := writesTo(body, req)
	if countEmpty(reqWrites) > 1 || len(reqWrites) != countEmpty(reqWrites) {
		return "the request " + req + " is changed after it is built"
	}
	return ""
}

func countEmpty(ss []string) int {
	n := 0
	for _, s := range ss {
		if s == "" {
			n++
		}
	}
	return n
}

// writesTo lists the writes to the variable name in body, outside nested
// function literals: "" for an assignment or address-of of the whole
// variable, or the field name for an assignment to one of its fields.
func writesTo(body *ast.BlockStmt, name string) []string {
	var out []string
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				switch l := lhs.(type) {
				case *ast.Ident:
					if l.Name == name {
						out = append(out, "")
					}
				case *ast.SelectorExpr:
					if identName(l.X) == name {
						out = append(out, l.Sel.Name)
					}
				}
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND && identName(n.X) == name {
				out = append(out, "&")
			}
		}
		return true
	})
	return out
}

// TestScanCallProblem checks the guard itself on the shapes it must refuse.
func TestScanCallProblem(t *testing.T) {
	head := "q, errClass, withheld, err := scopedQuery(ctx, rqr, req)\n"
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"the production shape", head + "scanHeaders(ctx, svc, q, errClass, req)", true},
		{"an offset", head + "q.Offset = 5\nscanHeaders(ctx, svc, q, errClass, req)", true},
		{"a literal request", head + "scanHeaders(ctx, svc, q, errClass, scopeRequest{Type: t})", false},
		{"another request", head + "scanHeaders(ctx, svc, q, errClass, other)", false},
		{"another error class", head + "scanHeaders(ctx, svc, q, errListLoad, req)", false},
		{"no scopedQuery", "scanHeaders(ctx, svc, q, errClass, req)", false},
		{"props dropped", head + "q.Props = nil\nscanHeaders(ctx, svc, q, errClass, req)", false},
		{"query replaced", head + "q = store.GraphQuery{}\nscanHeaders(ctx, svc, q, errClass, req)", false},
		{"query by pointer", head + "f(&q)\nscanHeaders(ctx, svc, q, errClass, req)", false},
		{"two scopedQuery calls", head + head + "scanHeaders(ctx, svc, q, errClass, req)", false},
		{"request changed", head + "req.Scope = nil\nscanHeaders(ctx, svc, q, errClass, req)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\nfunc f() {\n"+tc.body+"\n}", 0)
			if err != nil {
				t.Fatal(err)
			}
			body := file.Decls[0].(*ast.FuncDecl).Body
			calls := directCalls(body, "scanHeaders")
			if len(calls) != 1 {
				t.Fatalf("%d scanHeaders calls", len(calls))
			}
			if msg := scanCallProblem(body, calls[0]); (msg == "") != tc.ok {
				t.Errorf("problem = %q, want ok=%v", msg, tc.ok)
			}
		})
	}
}

type scopedQueryAssignment struct{ query, class, req string }

// scopedQueryAssignments lists the scopedQuery assignments in body, outside
// nested function literals.
func scopedQueryAssignments(body *ast.BlockStmt) []scopedQueryAssignment {
	var out []scopedQueryAssignment
	ast.Inspect(body, func(n ast.Node) bool {
		if _, lit := n.(*ast.FuncLit); lit {
			return false
		}
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 || len(as.Lhs) != 4 {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok || identName(call.Fun) != "scopedQuery" || len(call.Args) != 3 {
			return true
		}
		out = append(out, scopedQueryAssignment{
			query: identName(as.Lhs[0]), class: identName(as.Lhs[1]), req: identName(call.Args[2]),
		})
		return true
	})
	return out
}

// directCalls returns the calls of the package function name in body, not
// counting those inside nested function literals, which are checked on their
// own.
func directCalls(body *ast.BlockStmt, name string) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		if _, lit := n.(*ast.FuncLit); lit {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok && identName(call.Fun) == name {
			out = append(out, call)
		}
		return true
	})
	return out
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
