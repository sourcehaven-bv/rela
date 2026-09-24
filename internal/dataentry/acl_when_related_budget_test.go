package dataentry

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// An ACL `visible:` grant whose `when:` uses related() is answered once per
// page, not once per row (TKT-205V2N): the reads prime the grant traversals
// before they redact the rows.
func TestQueryBudget_ListPageACLRelatedWhenIsSizeIndependent(t *testing.T) {
	counts := make([]int, 0, 2)
	var detail string
	for _, n := range []int{10, 50} {
		app, counting, _, ctx := newBudgetAppOn(t, n, memstore.New())
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"}, Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"},
					{Field: "status", When: "not related(entity, 'implements', { title = 'Feature 1' })"},
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

		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100")
		if rec.Code != http.StatusOK || len(resp.Data) < 10 {
			t.Fatalf("list: %d, %d rows", rec.Code, len(resp.Data))
		}
		for _, row := range resp.Data {
			assertStatusVerdict(t, row)
		}
		counts = append(counts, counting.Calls()["MatchingIDs"])
		detail = counting.String()
	}
	if counts[0] != counts[1] || counts[1] != 1 {
		t.Fatalf("MatchingIDs = %d at 10 rows, %d at 50; want 1 at both (%s)", counts[0], counts[1], detail)
	}
}

// A single GET evaluates the grants several times (strip, `_fields`,
// `_relations`); the traversal behind them is answered once. The baseline is
// the same GET under a grant that does not traverse.
func TestQueryBudget_GetEntityACLRelatedWhenBindsOnce(t *testing.T) {
	get := func(when string) int {
		app, counting, _, ctx := newBudgetAppOn(t, 10, memstore.New())
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"}, Update: []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"},
					{Field: "status", When: when},
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
		rec := getEntityAs(ctx, t, app, d, "ticket", "tickets", "TKT-0001", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("get: %d %s", rec.Code, rec.Body)
		}
		return counting.Calls()["MatchingIDs"]
	}
	base := get("entity.title ~= ''")
	if n := get("not related(entity, 'implements', { title = 'Feature 1' })"); n != base+1 {
		t.Fatalf("MatchingIDs = %d, want %d (baseline %d plus one traversal)", n, base+1, base)
	}
}

// assertStatusVerdict checks a row's status visibility against the fixture:
// ticket i implements feature i%5+1, so every fifth ticket implements
// Feature 1 and has its status hidden.
func assertStatusVerdict(t *testing.T, row v1.Entity) {
	t.Helper()
	var i int
	if _, err := fmt.Sscanf(row.ID, "TKT-%04d", &i); err != nil {
		t.Fatal(err)
	}
	_, shown := row.Properties["status"]
	if want := i%5 != 0; shown != want {
		t.Errorf("%s: status shown = %v, want %v", row.ID, shown, want)
	}
}
