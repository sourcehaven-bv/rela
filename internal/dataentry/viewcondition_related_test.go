package dataentry

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// relatedListCondition narrows the budget list to tickets implementing
// Feature 1: every fifth ticket.
const relatedListCondition = "related(entity, 'implements', { title = 'Feature 1' })"

func withRelatedListCondition(t *testing.T, app *App) {
	t.Helper()
	l := app.Cfg().Lists["tickets"]
	l.Condition = relatedListCondition
	app.Cfg().Lists["tickets"] = l
	if err := app.SetViewConditions(AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatalf("wire view conditions: %v", err)
	}
}

// A list `condition:` using related() lowers to a store traversal on the
// page query (TKT-I39Y7N4): the store counts and pages it, so neither the
// calls nor the rows read grow with the list or the store.
func TestViewCondition_RelatedIsSizeIndependent(t *testing.T) {
	b, detail := measureBudget(func(n, pad int) (storetest.Cost, string) {
		app, counting, d, ctx := newBudgetAppPadded(t, n, pad, memstore.New(), budgetMeta())
		withRelatedListCondition(t, app)
		counting.Reset()
		assertRelatedConditionPage(ctx, t, app, d, counting, n, 0)
		return counting.Cost(), counting.String()
	})
	assertBudget(t, "related list condition", b, listPageBudget, detail)
}

// assertRelatedConditionPage lists the tickets under [relatedListCondition]
// and fails unless the store served the page: a fifth of the n fixture
// tickets, counted with CountMatched. matching is how many traversals the
// page may answer over its rows: 0 for the condition, which the store judges.
func assertRelatedConditionPage(
	ctx context.Context, t *testing.T, app *App, d *acl.Declarative, counting *storetest.Counting, n, matching int,
) {
	t.Helper()
	resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&list_id=tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != n/5 || resp.Meta.Total != n/5 {
		t.Fatalf("n=%d: rows = %d, total %d, want %d", n, len(resp.Data), resp.Meta.Total, n/5)
	}
	if calls := counting.Calls(); calls["MatchingFaces"] != matching || calls["CountMatched"] != 1 {
		t.Errorf("n=%d: the condition was not pushed down: %s", n, counting)
	}
}

// The condition's traversal is authorized by the reader's own gate: a reader
// who cannot read features sees no ticket "implementing Feature 1", because
// for them no such feature exists.
func TestViewCondition_RelatedUsesReaderGate(t *testing.T) {
	app, _, _, ctx := newBudgetAppOn(t, 10, memstore.New())
	withRelatedListCondition(t, app)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"T1": "reader"},
	}, app.store)
	setTestACL(t, app, d)
	resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&list_id=tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("rows = %d, want 0: the traversal must not see a feature the reader cannot read", len(resp.Data))
	}
}

// A list condition and a grant `when:` that both traverse stay size
// independent together. The condition reads no gated field, so it lowers
// (TKT-I39Y7N4); the grant is answered once per page (TKT-205V2N).
func TestViewCondition_RelatedWithTraversingGrantIsSizeIndependent(t *testing.T) {
	b, detail := measureBudget(func(n, pad int) (storetest.Cost, string) {
		app, counting, _, ctx := newBudgetAppPadded(t, n, pad, memstore.New(), budgetMeta())
		withRelatedListCondition(t, app)
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"},
					{Field: "status", When: "related(entity, 'implements', { title = 'Feature 2' })"},
				}},
			}},
			Assignments: map[string]string{"T1": "editor"},
		}, app.store)
		setTestACL(t, app, d)
		res, err := ResolverFromProfile("", app.Meta(), app.store, d, nil)
		if err != nil {
			t.Fatal(err)
		}
		setTestResolver(t, app, res)
		counting.Reset()
		assertRelatedConditionPage(ctx, t, app, d, counting, n, 1)
		return counting.Cost(), counting.String()
	})
	assertBudget(t, "related list condition with traversing grant", b, relatedGrantConditionBudget, detail)
}

// relatedGrantConditionBudget is listPageBudget plus the grant's batched
// traversal for the page (ONE MatchingFaces). Measured.
const relatedGrantConditionBudget = 7

// A traversal the gate refuses, rather than denies, fails the request. Here
// the condition filters on feature.title, which a `visible:` block hides from
// some role, so no reader may filter on it. An empty list would read as "no
// ticket implements Feature 1".
func TestViewCondition_RelatedUnsupportedFailsRequest(t *testing.T) {
	app, _, _, ctx := newBudgetAppOn(t, 10, memstore.New())
	withRelatedListCondition(t, app)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reader": {Read: []string{"*"}},
			"guest":  {Read: []string{"*"}, Visible: map[string][]acl.FieldGrant{"feature": {}}},
		},
		Assignments: map[string]string{"T1": "reader"},
	}, app.store)
	setTestACL(t, app, d)
	resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&list_id=tickets")
	if rec.Code < 400 || len(resp.Data) != 0 {
		t.Fatalf("status = %d with %d rows, want a 4xx/5xx and no rows", rec.Code, len(resp.Data))
	}
	if !strings.Contains(rec.Body.String(), "cannot be filtered on") {
		t.Fatalf("body = %s, want the traversal refusal", rec.Body)
	}
}
