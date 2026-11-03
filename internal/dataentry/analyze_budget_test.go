package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// analyzeBudget is the pinned store-read count of one _analyze request over
// the budget fixture. Before TKT-QM833U it grew with the rows: every
// validation candidate paid its local-role probes one at a time, and every
// violator was re-loaded with its own GetAddress.
const analyzeBudget = 9

// analyzeFieldRulesBudget is the same request under field rules, where the
// unprimed path cost 122 reads at 10 tickets and 522 at 50.
const analyzeFieldRulesBudget = 26

// analyzeBudgetMeta is [budgetMeta] with one validation rule that half the
// tickets break (status alternates open and done), so the validation check
// both lists candidates and re-loads violators.
func analyzeBudgetMeta() *metamodel.Metamodel {
	meta := budgetMeta()
	meta.Validations = []metamodel.ValidationRule{{
		Name: "ticket-done", Description: "A ticket must be done",
		EntityType: "ticket", Then: []string{"status=done"},
	}}
	return meta
}

// _analyze reads the whole graph, so its cost grows with the store by
// design: each check streams every row once. What must NOT grow is the
// number of store calls (TKT-QM833U): 12,000 statements per request on
// atlas came from a per-row local-role probe on the pushed-down list path
// and a per-violator re-load.
func TestQueryBudget_AnalyzeIsSizeIndependent(t *testing.T) {
	small, large, detail := readsForWith(t, func(*testing.T) store.Store { return memstore.New() },
		analyzeBudgetMeta, analyzeOp)
	assertBudget(t, "_analyze", small, large, analyzeBudget, detail)
}

// The realistic budget ACL has no field rules, so field redaction never asks
// for the requester's local roles and the per-row probe TKT-QM833U removed
// stays invisible under it. Atlas's acl.yaml has `visible:` blocks; so does
// this policy.
func TestQueryBudget_AnalyzeWithFieldRulesIsSizeIndependent(t *testing.T) {
	counts := make([]int, 0, 2)
	var detail string
	for _, n := range []int{10, 50} {
		app, counting, d, ctx := newFieldRulesBudgetApp(t, n)
		analyzeOp(t, app, d, ctx)
		counts = append(counts, counting.Reads())
		detail = counting.String()
	}
	if counts[0] != counts[1] {
		t.Errorf("_analyze under field rules: store reads grow with the rows: %d at 10, %d at 50 (%s)",
			counts[0], counts[1], detail)
	}
	if counts[1] != analyzeFieldRulesBudget {
		t.Errorf("_analyze under field rules: store reads = %d, pinned budget %d (%s)",
			counts[1], analyzeFieldRulesBudget, detail)
	}
}

// A caller that primed the rows itself (TKT-YLG695 primes at handler level)
// pays nothing more when the pushed-down list primes each page again: the
// local-role prime skips ids already primed for the request, and the
// traversal memo skips rows it holds. So the two primes can land in either
// order without double work.
func TestQueryBudget_PushdownOnPrimedCtxAddsNoPrimeReads(t *testing.T) {
	app, counting, d, ctx := newFieldRulesBudgetApp(t, 50)
	ctx = gateCtxFor(ctx, t, d)
	var rows []*entity.Entity
	for e, err := range counting.Unwrap().ListEntities(ctx, store.EntityQuery{Type: "ticket", Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, e)
	}
	list := func(ctx context.Context) int {
		counting.Reset()
		got := 0
		for _, err := range (lateGatedReader{app: app}).ListEntities(ctx, store.EntityQuery{Type: "ticket", Faces: store.AllFaces()}) {
			if err != nil {
				t.Fatal(err)
			}
			got++
		}
		if got != len(rows) {
			t.Fatalf("listed %d tickets, want %d", got, len(rows))
		}
		return counting.Reads()
	}
	unprimed := list(ctx)
	primed := list(primeVerdicts(ctx, app.fieldResolver, rows))
	if primed >= unprimed {
		t.Fatalf("a primed ctx cost %d reads, the unprimed one %d: the list primed its pages again", primed, unprimed)
	}
	if want := 1; primed != want {
		t.Errorf("a primed ctx cost %d reads, want %d (the list itself): %s", primed, want, counting)
	}
}

// newFieldRulesBudgetApp is the budget fixture of n tickets under
// [realisticBudgetPolicy] plus `visible:` grants, with the policy-backed
// field resolver installed, so every redacted row asks for local roles.
func newFieldRulesBudgetApp(t *testing.T, n int) (*App, *storetest.Counting, *acl.Declarative, context.Context) {
	t.Helper()
	app, counting, _, ctx := newBudgetAppWith(t, n, memstore.New(), analyzeBudgetMeta())
	useRealisticBudgetPolicy(t, app, counting)
	policy := realisticBudgetPolicy()
	editor := policy.Roles["editor"]
	editor.Visible = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}, {Field: "status"}}}
	policy.Roles["editor"] = editor
	assignee := policy.Roles["assignee"]
	assignee.Visible = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}
	policy.Roles["assignee"] = assignee
	d := mustNewACL(t, policy, app.store)
	setTestACL(t, app, d)
	resolver, err := ResolverFromProfile("", app.Meta(), app.store, d)
	if err != nil {
		t.Fatal(err)
	}
	setTestResolver(t, app, resolver)
	counting.Reset()
	return app, counting, d, ctx
}

var analyzeOp budgetOp = func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_analyze", http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1Analyze(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("_analyze: %d %s", rec.Code, rec.Body)
	}
	var result APIAnalysisResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode _analyze: %v", err)
	}
	// The rule must have fired, or the re-load path was not measured.
	violations := 0
	for _, iss := range result.Issues {
		if iss.Message == "A ticket must be done" {
			violations++
		}
	}
	if violations < 5 {
		t.Fatalf("expected the ticket-done rule to report violators, got %d (%s)", violations, rec.Body)
	}
}
