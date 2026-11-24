package dataentry

import (
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// toggleMeta is [budgetMeta] with ticket status a state machine: closing a
// ticket needs it to implement Feature 1, reopening is free.
func toggleMeta() *metamodel.Metamodel {
	m := budgetMeta()
	m.Types = map[string]metamodel.CustomType{"work-status": {
		Values: []string{"open", "done"}, Initial: "open",
		Transitions: []metamodel.TransitionDef{
			{From: "open", To: "done", When: "related(entity, 'implements', { title = 'Feature 1' })"},
			{From: "done", To: "open"},
		},
	}}
	def := m.Entities["ticket"]
	def.Properties["status"] = metamodel.PropertyDef{Type: "work-status"}
	m.Entities["ticket"] = def
	return m
}

// toggleRole is the fixture's editor with the given field grants.
func toggleRole(fields, visible []acl.FieldGrant) *acl.Policy {
	role := acl.RoleDef{
		Read: []string{"*"}, Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"},
		Visible: map[string][]acl.FieldGrant{"ticket": visible},
	}
	if fields != nil {
		role.Fields = map[string][]acl.FieldGrant{"ticket": fields}
	}
	return &acl.Policy{Roles: map[string]acl.RoleDef{"editor": role}, Assignments: map[string]string{"T1": "editor"}}
}

var toggleColumn = dataentryconfig.ListColumn{
	Property: "status", Widget: dataentryconfig.WidgetToggle, On: "done", Off: "open",
}

// toggleListAs builds the toggle fixture with n tickets under policy and
// lists the tickets with rawQuery. Ticket i implements Feature i%5+1 and is
// done when i is odd.
func toggleListAs(
	t *testing.T, n, pad int, policy *acl.Policy, rawQuery string,
) (v1.ListResponse, *storetest.Counting) {
	t.Helper()
	app, counting, _, ctx := newBudgetAppPadded(t, n, pad, memstore.New(), toggleMeta())
	cfg := app.State().Cfg
	list := cfg.Lists["tickets"]
	list.Columns = append([]dataentryconfig.ListColumn{{Property: "title"}, toggleColumn}, list.Columns[2:]...)
	cfg.Lists["tickets"] = list
	cfg.Lists["features"] = dataentryconfig.List{EntityType: "feature", Columns: []dataentryconfig.ListColumn{
		{Property: "title"}, toggleColumn,
	}}
	d := mustNewACL(t, policy, app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", app.Meta(), app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	setTestResolver(t, app, res)
	counting.Reset()
	resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", rawQuery)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	return resp, counting
}

func togglesByID(resp v1.ListResponse) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range resp.Data {
		out[row.ID] = row.Toggles
	}
	return out
}

func TestRowToggles(t *testing.T) {
	all := []acl.FieldGrant{{Field: "title"}, {Field: "status"}}
	tests := []struct {
		name   string
		policy *acl.Policy
		query  string
		want   map[string]any // per ticket: the status toggle, nil for none
	}{
		{
			// TKT-0010 is open and implements Feature 1: it may close.
			// TKT-0002 is open without it: the transition is refused.
			// TKT-0001 is done: it may reopen.
			name: "allowed and denied transition", policy: toggleRole(nil, all), query: "list_id=tickets",
			want: map[string]any{"TKT-0001": "open", "TKT-0002": nil, "TKT-0010": "done"},
		},
		{
			name: "denied by field grant", policy: toggleRole([]acl.FieldGrant{{Field: "title"}}, all),
			query: "list_id=tickets", want: map[string]any{"TKT-0001": nil, "TKT-0005": nil},
		},
		{
			name: "hidden field", policy: toggleRole(nil, []acl.FieldGrant{{Field: "title"}}),
			query: "list_id=tickets", want: map[string]any{"TKT-0001": nil, "TKT-0005": nil},
		},
		{
			name: "no list_id", policy: toggleRole(nil, all), query: "",
			want: map[string]any{"TKT-0001": nil, "TKT-0005": nil},
		},
		{
			name: "list of another type", policy: toggleRole(nil, all), query: "list_id=features",
			want: map[string]any{"TKT-0001": nil, "TKT-0005": nil},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := toggleListAs(t, 10, 0, tc.policy, tc.query)
			got := togglesByID(resp)
			for id, want := range tc.want {
				row, ok := got[id]
				if !ok {
					t.Fatalf("%s not served", id)
				}
				if want == nil {
					if _, has := row["status"]; has {
						t.Errorf("%s: _toggles = %v, want no status entry", id, row)
					}
					continue
				}
				if row["status"] != want {
					t.Errorf("%s: _toggles = %v, want status %v", id, row, want)
				}
			}
		})
	}
}

// toggleCurrent and toggleTarget, including an unset value and each other mode.
func TestToggleTarget(t *testing.T) {
	c := dataentryconfig.ListColumn{Property: "s", On: "done", Off: "open"}
	tests := []struct {
		other, current, want string
		flips                bool
	}{
		{"", "done", "open", true},
		{"", "open", "done", true},
		{dataentryconfig.ToggleOtherDisabled, "doing", "", false},
		{dataentryconfig.ToggleOtherHidden, "doing", "", false},
		{dataentryconfig.ToggleOtherOn, "doing", "open", true},
	}
	for _, tc := range tests {
		c.Other = tc.other
		got, flips := toggleTarget(c, tc.current)
		if got != tc.want || flips != tc.flips {
			t.Errorf("other %q current %q: got %q %v, want %q %v", tc.other, tc.current, got, flips, tc.want, tc.flips)
		}
	}
	e := entity.New("X", "ticket")
	if got := toggleCurrent(e, c, ""); got != "open" {
		t.Errorf("unset = %q, want off", got)
	}
	if got := toggleCurrent(e, c, "doing"); got != "doing" {
		t.Errorf("unset machine = %q, want entry value", got)
	}
	e.Properties["s"] = true
	if got := toggleCurrent(e, c, ""); got != "true" {
		t.Errorf("bool = %q, want true", got)
	}
}

// The toggle extra binds the transition `related(...)` once per page: store
// reads do not grow with the page, and rows read do not grow with the store.
func TestQueryBudget_ListToggleIsSizeIndependent(t *testing.T) {
	policy := func() *acl.Policy { return toggleRole(nil, []acl.FieldGrant{{Field: "title"}, {Field: "status"}}) }
	run := func(n, pad int) *storetest.Counting {
		resp, counting := toggleListAs(t, n, pad, policy(), "list_id=tickets&per_page=100")
		served := 0
		for _, row := range resp.Data {
			if row.Toggles != nil {
				served++
			}
		}
		if served == 0 {
			t.Fatal("no row carries _toggles, so this budget proves nothing")
		}
		return counting
	}
	small, large := run(10, 0), run(50, 0)
	if small.Cost().Calls != large.Cost().Calls {
		t.Fatalf("store reads grow with page size: %d at 10 rows, %d at 50 (%s)",
			small.Cost().Calls, large.Cost().Calls, large.String())
	}
	storetest.AssertScaleFree(t, func(pad int) (storetest.Cost, string) {
		counting := run(10, pad)
		return counting.Cost(), counting.String()
	})
}
