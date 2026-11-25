package dataentry

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// groupedList is a computed grouping over the budget fixture: ticket i
// implements feature i%5+1 and is open when i is even.
var groupedList = dataentryconfig.List{EntityType: "ticket", GroupBy: &dataentryconfig.ListGroupBy{
	Groups: []dataentryconfig.ListGroup{
		{ID: "feature-one", Condition: "related(entity, 'implements', { title = 'Feature 1' })"},
		{ID: "open", Condition: "entity.status == 'open'"},
		{ID: "rest"},
	},
}}

// newGroupedListApp is the budget app with groupedList configured as
// "grouped" and the policy installed; visible limits the ticket fields the
// reader sees, nil for all.
func newGroupedListApp(
	t *testing.T, n int, visible []acl.FieldGrant,
) (counting *storetest.Counting, list func(query string) (v1.ListResponse, int)) {
	t.Helper()
	app, c, _, ctx := newBudgetAppOn(t, n, memstore.New())
	app.Cfg().Lists["grouped"] = groupedList
	wireRealViewConditions(t, app)
	role := acl.RoleDef{Read: []string{"*"}}
	if visible != nil {
		role.Visible = map[string][]acl.FieldGrant{"ticket": visible}
	}
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": role},
		Assignments: map[string]string{"T1": "reader"},
	}, app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", app.Meta(), app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	setTestResolver(t, app, res)
	list = func(query string) (v1.ListResponse, int) {
		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", query)
		return resp, rec.Code
	}
	return c, list
}

func wantGroup(i int, statusVisible bool) string {
	switch {
	case i%5 == 0:
		return "feature-one"
	case statusVisible && i%2 == 0:
		return "open"
	default:
		return "rest"
	}
}

func TestListGroups_StampsFirstMatchingGroup(t *testing.T) {
	_, list := newGroupedListApp(t, 10, nil)
	resp, code := list("per_page=100&list_id=grouped")
	if code != http.StatusOK || len(resp.Data) != 10 {
		t.Fatalf("list: %d, %d rows", code, len(resp.Data))
	}
	for _, row := range resp.Data {
		var i int
		if _, err := fmt.Sscanf(row.ID, "TKT-%04d", &i); err != nil {
			t.Fatal(err)
		}
		if want := wantGroup(i, true); row.Group != want {
			t.Errorf("%s: _group = %q, want %q", row.ID, row.Group, want)
		}
	}

	resp, _ = list("per_page=100")
	for _, row := range resp.Data {
		if row.Group != "" {
			t.Fatalf("%s: _group %q without list_id", row.ID, row.Group)
		}
	}
}

// A condition over a field the reader cannot see places no row by its value:
// status binds nil, so no row lands in "open".
func TestListGroups_HiddenFieldDoesNotPlaceRow(t *testing.T) {
	_, list := newGroupedListApp(t, 10, []acl.FieldGrant{{Field: "title"}})
	resp, code := list("per_page=100&list_id=grouped")
	if code != http.StatusOK || len(resp.Data) != 10 {
		t.Fatalf("list: %d, %d rows", code, len(resp.Data))
	}
	for _, row := range resp.Data {
		var i int
		if _, err := fmt.Sscanf(row.ID, "TKT-%04d", &i); err != nil {
			t.Fatal(err)
		}
		if want := wantGroup(i, false); row.Group != want {
			t.Errorf("%s: _group = %q, want %q", row.ID, row.Group, want)
		}
	}
}

// Grouping costs one batched traversal per page, whatever its size.
func TestQueryBudget_ListGroupsIsSizeIndependent(t *testing.T) {
	calls := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		counting, list := newGroupedListApp(t, n, nil)
		counting.Reset()
		if _, code := list("per_page=100&list_id=grouped"); code != http.StatusOK {
			t.Fatalf("list: %d", code)
		}
		calls = append(calls, counting.Calls()["MatchingFaces"])
	}
	if calls[0] != calls[1] {
		t.Fatalf("MatchingFaces = %d at 10 rows, %d at 50; want equal", calls[0], calls[1])
	}
}

// Without a fallback, a row that matches no group carries no `_group`.
func TestListGroups_NoFallbackLeavesRowUngrouped(t *testing.T) {
	saved := groupedList.GroupBy.Groups
	groupedList.GroupBy.Groups = saved[:2]
	t.Cleanup(func() { groupedList.GroupBy.Groups = saved })
	_, list := newGroupedListApp(t, 10, nil)
	resp, code := list("per_page=100&list_id=grouped")
	if code != http.StatusOK {
		t.Fatalf("list: %d", code)
	}
	for _, row := range resp.Data {
		var i int
		if _, err := fmt.Sscanf(row.ID, "TKT-%04d", &i); err != nil {
			t.Fatal(err)
		}
		want := wantGroup(i, true)
		if want == "rest" {
			want = ""
		}
		if row.Group != want {
			t.Errorf("%s: _group = %q, want %q", row.ID, row.Group, want)
		}
	}
}
