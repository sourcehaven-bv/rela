package dataentry

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
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

// A list `condition:` using related() is answered for the whole candidate
// set at once: one store query, whatever the list's size (TKT-205V2N).
func TestViewCondition_RelatedIsSizeIndependent(t *testing.T) {
	counts := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		app, counting, d, ctx := newBudgetAppOn(t, n, memstore.New())
		withRelatedListCondition(t, app)
		counting.Reset()
		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&list_id=tickets")
		if rec.Code != http.StatusOK {
			t.Fatalf("list: %d %s", rec.Code, rec.Body)
		}
		if len(resp.Data) != n/5 {
			t.Fatalf("n=%d: rows = %d, want %d", n, len(resp.Data), n/5)
		}
		counts = append(counts, counting.Calls()["MatchingIDs"])
	}
	if counts[0] != counts[1] || counts[1] != 1 {
		t.Fatalf("MatchingIDs = %v, want 1 at both sizes", counts)
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
	app.acl = d
	resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&list_id=tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("rows = %d, want 0: the traversal must not see a feature the reader cannot read", len(resp.Data))
	}
}

// A list condition and a grant `when:` that both traverse stay size
// independent together: every per-row redaction on the list path reads
// primed answers (TKT-205V2N).
func TestViewCondition_RelatedWithTraversingGrantIsSizeIndependent(t *testing.T) {
	counts := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		app, counting, _, ctx := newBudgetAppOn(t, n, memstore.New())
		withRelatedListCondition(t, app)
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"},
					{Field: "status", When: "not related(entity, 'implements', { title = 'Feature 2' })"},
				}},
			}},
			Assignments: map[string]string{"T1": "editor"},
		}, app.store)
		app.acl = d
		res, err := ResolverFromProfile("", app.Meta(), app.store, d)
		if err != nil {
			t.Fatal(err)
		}
		app.fieldResolver = res
		counting.Reset()
		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&list_id=tickets")
		if rec.Code != http.StatusOK || len(resp.Data) != n/5 {
			t.Fatalf("n=%d: %d, %d rows, want %d", n, rec.Code, len(resp.Data), n/5)
		}
		counts = append(counts, counting.Calls()["MatchingIDs"])
	}
	if counts[0] != counts[1] {
		t.Fatalf("MatchingIDs = %v, want the same at both sizes", counts)
	}
}

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
	app.acl = d
	resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&list_id=tickets")
	if rec.Code < 400 || len(resp.Data) != 0 {
		t.Fatalf("status = %d with %d rows, want a 4xx/5xx and no rows", rec.Code, len(resp.Data))
	}
	if !strings.Contains(rec.Body.String(), "cannot be filtered on") {
		t.Fatalf("body = %s, want the traversal refusal", rec.Body)
	}
}
