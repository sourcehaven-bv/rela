package dataentry

import (
	"net/http"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func TestWantRowFields(t *testing.T) {
	tests := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{"status", []string{"status"}},
		{" status , done,status,,", []string{"status", "done"}},
	}
	for _, tc := range tests {
		got := wantRowFields(map[string][]string{"field_verdicts": {tc.raw}})
		if !slices.Equal(got, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.raw, got, tc.want)
		}
	}
	many := "a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r"
	if n := len(wantRowFields(map[string][]string{"field_verdicts": {many}})); n != maxRowFields {
		t.Errorf("got %d names, want the cap %d", n, maxRowFields)
	}
}

// `?field_verdicts=` gives each list row the verdicts of the named properties
// (TKT-M0AT73B): `_redacted` names a hidden one, `_fields` marks a
// read-only one, and the page answers both without a query per row.
func TestQueryBudget_ListRowFields(t *testing.T) {
	run := func(n, pad int, update []string) (*storetest.Counting, []rowFieldsView) {
		app, counting, _, ctx := newBudgetAppPadded(t, n, pad, memstore.New(), budgetMeta())
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"}, Update: update,
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"},
					{Field: "status", When: "related(entity, 'implements', { title = 'Feature 1' })"},
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
		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&field_verdicts=status")
		if rec.Code != http.StatusOK || len(resp.Data) < 10 {
			t.Fatalf("list: %d, %d rows", rec.Code, len(resp.Data))
		}
		views := make([]rowFieldsView, 0, len(resp.Data))
		for _, row := range resp.Data {
			if row.FieldAffordances == nil || row.Redacted == nil {
				t.Fatalf("%s: no _fields or _redacted", row.ID)
			}
			_, shown := row.Properties["status"]
			v := rowFieldsView{id: row.ID, shown: shown, redacted: slices.Contains(*row.Redacted, "status")}
			// A row the reader may not update says so in `_actions`; the
			// SPA reads that before `_fields`.
			v.readOnly = !row.Actions["update"]
			if fa, ok := (*row.FieldAffordances)["status"]; ok && fa.Writable != nil && !*fa.Writable {
				v.readOnly = true
			}
			views = append(views, v)
		}
		return counting, views
	}

	_, views := run(10, 0, []string{"ticket"})
	for _, v := range views {
		if v.redacted == v.shown {
			t.Errorf("%s: shown=%v but redacted=%v", v.id, v.shown, v.redacted)
		}
	}
	_, views = run(10, 0, nil)
	for _, v := range views {
		if v.shown && !v.readOnly {
			t.Errorf("%s: status visible without update rights but not marked read-only", v.id)
		}
	}

	var calls []int
	for _, n := range []int{10, 50} {
		counting, _ := run(n, 0, []string{"ticket"})
		total := 0
		for _, c := range counting.Calls() {
			total += c
		}
		calls = append(calls, total)
	}
	if calls[0] != calls[1] {
		t.Fatalf("store calls = %d at 10 rows, %d at 50; want equal", calls[0], calls[1])
	}
	storetest.AssertScaleFree(t, func(pad int) (storetest.Cost, string) {
		counting, _ := run(10, pad, []string{"ticket"})
		return counting.Cost(), counting.String()
	})
}

type rowFieldsView struct {
	id                        string
	shown, redacted, readOnly bool
}
