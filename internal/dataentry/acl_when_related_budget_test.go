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
		setTestACL(t, app, d)
		res, err := ResolverFromProfile("", app.Meta(), app.store, d)
		if err != nil {
			t.Fatal(err)
		}
		setTestResolver(t, app, res)
		counting.Reset()

		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100")
		if rec.Code != http.StatusOK || len(resp.Data) < 10 {
			t.Fatalf("list: %d, %d rows", rec.Code, len(resp.Data))
		}
		for _, row := range resp.Data {
			assertStatusVerdict(t, row)
		}
		counts = append(counts, counting.Calls()["MatchingFaces"])
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
		setTestACL(t, app, d)
		res, err := ResolverFromProfile("", app.Meta(), app.store, d)
		if err != nil {
			t.Fatal(err)
		}
		setTestResolver(t, app, res)
		counting.Reset()
		rec := getEntityAs(ctx, t, app, d, "ticket", "tickets", "TKT-0001", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("get: %d %s", rec.Code, rec.Body)
		}
		return counting.Calls()["MatchingFaces"]
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

// A list `?q=` under a field-hiding policy (BUG-WJSYQT) drops a hit whose
// text matched only fields the reader cannot see, which needs a field verdict
// per hit. The list's own reads (headers, relations) do not grow with the
// number of hits. Two per-hit terms remain, both inside the shared
// search.FieldVisibleSearcher contract that /_search and the MCP searcher use
// too: the searcher loads each hit once to tell which fields matched, and a
// `when:` that traverses is answered per hit because the hidden-field
// callback has no batch hook. BUG-57A53F tracks removing them; until
// then this pins them at exactly one each, so they cannot grow further.
func TestQueryBudget_ListFreeTextUnderFieldHiding(t *testing.T) {
	type sample struct{ fixed, entityLoads, traversals int }
	var got []sample
	var detail string
	for _, n := range []int{10, 50} {
		app, counting, _, ctx := newBudgetAppOn(t, n, memstore.New())
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"},
					{Field: "status", When: "not related(entity, 'implements', { title = 'Feature 1' })"},
				}},
			}},
			Assignments: map[string]string{"T1": "editor"},
		}, app.store)
		setTestACL(t, app, d)
		res, err := ResolverFromProfile("", app.Meta(), app.store, d)
		if err != nil {
			t.Fatal(err)
		}
		setTestResolver(t, app, res)
		counting.Reset()

		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "q=Ticket&per_page=100")
		if rec.Code != http.StatusOK || len(resp.Data) != n {
			t.Fatalf("list ?q=: %d, %d rows, want %d (%s)", rec.Code, len(resp.Data), n, rec.Body)
		}
		calls := counting.Calls()
		got = append(got, sample{
			fixed:       counting.Reads() - calls["GetEntity"] - calls["MatchingFaces"],
			entityLoads: calls["GetEntity"] - n, // the in-memory searcher's own scan reads each row once
			// One batched traversal primes the served page; the rest are per hit.
			traversals: calls["MatchingFaces"] - 1,
		})
		detail = counting.String()
	}
	if got[0].fixed != got[1].fixed {
		t.Errorf("list reads grow with hits: %d at 10, %d at 50 (%s)", got[0].fixed, got[1].fixed, detail)
	}
	for i, n := range []int{10, 50} {
		if got[i].entityLoads != n || got[i].traversals != n {
			t.Errorf("at %d hits: %d field-check entity loads and %d traversals, want %d each (%s)",
				n, got[i].entityLoads, got[i].traversals, n, detail)
		}
	}
}
