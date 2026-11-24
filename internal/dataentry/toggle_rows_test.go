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

func togglesByID(resp v1.ListResponse) map[string]map[string]v1.Toggle {
	out := map[string]map[string]v1.Toggle{}
	for _, row := range resp.Data {
		out[row.ID] = row.Toggles
	}
	return out
}

// none marks a row whose status has no `_toggles` entry.
var none = &v1.Toggle{}

func TestRowToggles(t *testing.T) {
	all := []acl.FieldGrant{{Field: "title"}, {Field: "status"}}
	tests := []struct {
		name   string
		policy *acl.Policy
		query  string
		want   map[string]*v1.Toggle // per ticket; none for no entry
	}{
		{
			// TKT-0010 is open and implements Feature 1: it may close.
			// TKT-0002 is open without it: the transition is refused.
			// TKT-0001 is done: it may reopen.
			name: "allowed and denied transition", policy: toggleRole(nil, all), query: "list_id=tickets",
			want: map[string]*v1.Toggle{
				"TKT-0001": {Checked: true, To: "open"}, "TKT-0002": {Checked: false},
				"TKT-0010": {Checked: false, To: "done"},
			},
		},
		{
			name: "denied by field grant", policy: toggleRole([]acl.FieldGrant{{Field: "title"}}, all),
			query: "list_id=tickets",
			want:  map[string]*v1.Toggle{"TKT-0001": {Checked: true}, "TKT-0010": {Checked: false}},
		},
		{
			name: "hidden field", policy: toggleRole(nil, []acl.FieldGrant{{Field: "title"}}),
			query: "list_id=tickets", want: map[string]*v1.Toggle{"TKT-0001": none, "TKT-0010": none},
		},
		{
			name: "no list_id", policy: toggleRole(nil, all), query: "",
			want: map[string]*v1.Toggle{"TKT-0001": none, "TKT-0010": none},
		},
		{
			name: "list of another type", policy: toggleRole(nil, all), query: "list_id=features",
			want: map[string]*v1.Toggle{"TKT-0001": none, "TKT-0010": none},
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
				cell, has := row["status"]
				if want == none {
					if has {
						t.Errorf("%s: _toggles = %v, want no status entry", id, row)
					}
					continue
				}
				if !has || cell != *want {
					t.Errorf("%s: _toggles = %v, want status %+v", id, row, *want)
				}
			}
		})
	}
}

// A row on a named face cannot answer a transition `when:` that follows a
// relation, so its toggle is always disabled; it still shows its state.
func TestRowToggles_FacedRowIsDisabled(t *testing.T) {
	app, _, _, ctx := newBudgetAppPadded(t, 10, 0, memstore.New(), toggleMeta())
	d := mustNewACL(t, toggleRole(nil, []acl.FieldGrant{{Field: "title"}, {Field: "status"}}), app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", app.Meta(), app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	setTestResolver(t, app, res)
	src := toggleSources{schema: app.State(), listType: "ticket", res: res, svc: app.affordances}
	row := func(face entity.Face) *entity.Entity {
		e := entity.New("TKT-0010", "ticket") // open, implements Feature 1
		e.Face = face
		e.SetString("title", "x")
		e.SetString("status", "open")
		return e
	}
	rows := []*entity.Entity{row(""), row("draft")}
	data := make([]v1.Entity, len(rows))
	serveRowToggles(gateCtxFor(ctx, t, d), src, []dataentryconfig.ListColumn{toggleColumn}, rows, data)
	if got := data[0].Toggles["status"]; got != (v1.Toggle{To: "done"}) {
		t.Errorf("default face: %+v, want an allowed close", got)
	}
	if got, ok := data[1].Toggles["status"]; !ok || got != (v1.Toggle{}) {
		t.Errorf("named face: %+v (present %v), want an unchecked, disabled entry", got, ok)
	}
}

// A boolean toggle reads a bool and a string alike, and writes a JSON bool.
func TestRowToggles_Boolean(t *testing.T) {
	c := dataentryconfig.ListColumn{Property: "b", On: "true", Off: "false"}
	for _, v := range []any{true, "true", "TRUE", "1"} {
		e := entity.New("X", "ticket")
		e.Properties["b"] = v
		if got := toggleCurrent(e, c, "", true); got != "true" {
			t.Errorf("%#v reads as %q, want true", v, got)
		}
	}
}

// A row of a variant of a variant of the list's type belongs to the list.
func TestToggleSources_IsListType(t *testing.T) {
	src := toggleSources{listType: "base", extends: map[string]string{"mid": "base"}}
	if !src.isListType("leaf", &metamodel.EntityDef{Extends: "mid"}) {
		t.Error("a two-level variant must count")
	}
	if src.isListType("other", &metamodel.EntityDef{Extends: "elsewhere"}) {
		t.Error("an unrelated type must not count")
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
	if got := toggleCurrent(e, c, "", false); got != "open" {
		t.Errorf("unset = %q, want off", got)
	}
	if got := toggleCurrent(e, c, "doing", false); got != "doing" {
		t.Errorf("unset machine = %q, want entry value", got)
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
