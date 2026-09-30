package dataentry

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestQueryBudget_ScopedVerdictListPageIsSizeIndependent is the list page
// under a scoped read verdict: P1 reads tickets only through a conferring
// `watches` edge, so the row gate and the neighbour gate (visibleRelationIDs,
// whose TKT-0001 neighbour is a ticket) both run MatchingFaces. Each must
// stay one query per batch, at 10 rows and at 50.
//
// GetRelation is excluded: the per-row write affordances resolve the
// conferred role with one GetRelation per row (TKT-0ZXW0Z). That cost
// predates the per-face gate and is not a read-gate cost.
func TestQueryBudget_ScopedVerdictListPageIsSizeIndependent(t *testing.T) {
	reads := make([]int, 0, 2)
	matching := make([]int, 0, 2)
	var detail string
	for _, n := range []int{10, 50} {
		app, counting, _, ctx := newBudgetAppOn(t, n, memstore.New())
		for i := 1; i <= n; i++ {
			if _, err := app.store.CreateRelation(ctx,
				entity.RelationKey{From: "P1", Type: "watches", To: fmt.Sprintf("TKT-%04d", i)}, nil); err != nil {
				t.Fatal(err)
			}
		}
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"viewer":  {Read: []string{"feature", "person", "team", "epic", "program"}},
				"watcher": {Read: []string{"ticket"}},
			},
			Assignments:   map[string]string{"T1": "viewer"},
			RoleRelations: map[string]acl.RoleRelationDef{"watches": {Confers: "watcher"}},
		}, app.store)
		app.acl = d
		counting.Reset()

		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100")
		if rec.Code != http.StatusOK {
			t.Fatalf("list: %d %s", rec.Code, rec.Body)
		}
		if len(resp.Data) != n {
			t.Fatalf("list returned %d rows, want %d", len(resp.Data), n)
		}
		reads = append(reads, counting.Reads()-counting.Calls()["GetRelation"])
		matching = append(matching, counting.Calls()["MatchingFaces"])
		detail = counting.String()
	}
	if reads[0] != reads[1] {
		t.Errorf("store reads grow with page size: %d at 10 rows, %d at 50 (%s)", reads[0], reads[1], detail)
	}
	if matching[0] != matching[1] || matching[1] == 0 {
		t.Errorf("MatchingFaces calls = %v, want equal and non-zero (%s)", matching, detail)
	}
}
