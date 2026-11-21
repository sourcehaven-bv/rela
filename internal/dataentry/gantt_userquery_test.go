package dataentry

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// seedSearchTree seeds a root project with two child projects and an epic:
//
//	PRJ-A Alpha (open)
//	  PRJ-B Bravo (done)   2026-01-10 .. 2026-03-01
//	  PRJ-C Charlie (open) 2026-02-01 .. 2026-04-01
//	  EPIC-1 Delta         2026-05-01 .. 2026-09-30
func seedSearchTree(app *App) {
	seedProject(app, "PRJ-A", "Alpha", map[string]any{"status": "open"})
	seedProject(app, "PRJ-B", "Bravo", map[string]any{
		"status": "done", "planned_start": "2026-01-10", "planned_end": "2026-03-01"})
	seedProject(app, "PRJ-C", "Charlie", map[string]any{
		"status": "open", "planned_start": "2026-02-01", "planned_end": "2026-04-01"})
	seedEpic(app, "EPIC-1", "Delta", "2026-05-01", "2026-09-30")
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-B"})
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "contains", To: "PRJ-C"})
	seedRelation(app, &entity.Relation{From: "PRJ-A", Type: "has-epic", To: "EPIC-1"})
}

func ganttIDs(resp v1.GanttResponse) []string {
	var ids []string
	var walk func(ns []v1.GanttNode)
	walk = func(ns []v1.GanttNode) {
		for _, n := range ns {
			ids = append(ids, n.ID)
			walk(n.Children)
		}
	}
	walk(resp.Roots)
	return ids
}

func TestGantt_UserQuery(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"no query keeps the tree", "plan", "PRJ-A,EPIC-1,PRJ-B,PRJ-C"},
		{"search matches a property", "plan?q=bravo", "PRJ-B"},
		{"search matches an id", "plan?q=epic-1", "EPIC-1"},
		{"search with no match", "plan?q=zzz", ""},
		// The epic has no status property: a filter on a property a type
		// lacks does not narrow that type. Roots sort by id.
		{"filter keeps matches only", "plan?filter[status]=done", "EPIC-1,PRJ-B"},
		{"filter operator", "plan?filter[status][in]=done,open", "PRJ-A,EPIC-1,PRJ-B,PRJ-C"},
		{"search and filter combine", "plan?q=charlie&filter[status]=open", "PRJ-C"},
		{"empty filter is no constraint", "plan?filter[status]=", "PRJ-A,EPIC-1,PRJ-B,PRJ-C"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newGanttTestApp(t)
			seedSearchTree(app)
			got := strings.Join(ganttIDs(decodeGantt(t, ganttGet(context.Background(), app, tc.query))), ",")
			if got != tc.want {
				t.Errorf("ids = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGantt_UserQueryFiltersBeforeRollup pins the gate-before-fold order for
// the reader's own filters: a dropped child contributes nothing to its
// parent's rolled span.
func TestGantt_UserQueryFiltersBeforeRollup(t *testing.T) {
	app := newGanttTestApp(t)
	seedSearchTree(app)

	// Unfiltered: the epic stretches Alpha to 2026-09-30.
	all := decodeGantt(t, ganttGet(context.Background(), app, "plan"))
	if all.Roots[0].Rolled == nil || all.Roots[0].Rolled.End != "2026-09-30" {
		t.Fatalf("unfiltered rolled = %+v, want end 2026-09-30", all.Roots[0].Rolled)
	}

	// Searching for the project id prefix keeps the three projects and
	// drops the epic.
	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan?q=PRJ-"))
	root := resp.Roots[0]
	if root.ID != "PRJ-A" {
		t.Fatalf("root = %s, want PRJ-A", root.ID)
	}
	if root.Rolled == nil || root.Rolled.End != "2026-04-01" {
		t.Errorf("rolled = %+v, want end 2026-04-01: the dropped epic must not widen it", root.Rolled)
	}
	if strings.Contains(ganttBody(t, resp), "EPIC-1") {
		t.Errorf("dropped entity appears in the response")
	}
}

// TestGantt_UserQueryOrphansBecomeRoots pins the documented semantics: a node
// whose parent did not match has no parent to hang under, so it is a root.
func TestGantt_UserQueryOrphansBecomeRoots(t *testing.T) {
	app := newGanttTestApp(t)
	seedSearchTree(app)
	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan?q=Delta"))
	if len(resp.Roots) != 1 || resp.Roots[0].ID != "EPIC-1" {
		t.Errorf("roots = %v, want only EPIC-1", ganttIDs(resp))
	}
}

func TestGantt_UserQueryBadFilter(t *testing.T) {
	cases := []struct {
		name, query string
	}{
		{"unknown property", "plan?filter[ghost]=1"},
		{"unknown operator", "plan?filter[status][bogus]=1"},
		{"malformed key", "plan?filter[status][a][b]=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newGanttTestApp(t)
			seedSearchTree(app)
			rec := ganttGet(context.Background(), app, tc.query)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400; body=%s", rec.Code, rec.Body)
			}
		})
	}
}

// TestGantt_UserQueryDrillKeepsRoot: the drilled root stays whether or not it
// matches, and its matching descendants still show.
func TestGantt_UserQueryDrillKeepsRoot(t *testing.T) {
	app := newGanttTestApp(t)
	seedSearchTree(app)
	q := "plan?root=PRJ-A&q=" + url.QueryEscape("charlie")
	resp := decodeGantt(t, ganttGet(context.Background(), app, q))
	if got := strings.Join(ganttIDs(resp), ","); got != "PRJ-A,PRJ-C" {
		t.Errorf("ids = %q, want PRJ-A,PRJ-C", got)
	}
}

// TestGantt_UserQueryHiddenValueDoesNotMatch: a value the reader may not see
// is redacted before the search runs, so it matches nothing.
func TestGantt_UserQueryHiddenValueDoesNotMatch(t *testing.T) {
	app := newGanttTestApp(t)
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"status": false},
	}})
	seedSearchTree(app)

	resp := decodeGantt(t, ganttGet(context.Background(), app, "plan?q=done"))
	if len(resp.Roots) != 0 {
		t.Errorf("LEAK: search matched a redacted value: %v", ganttIDs(resp))
	}
}

// TestGantt_UserQueryHiddenRowDoesNotMatch: a row the reader may not read is
// not in the set, so a search for it finds nothing.
func TestGantt_UserQueryHiddenRowDoesNotMatch(t *testing.T) {
	app := newGanttTestApp(t)
	seedSearchTree(app)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"project"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	resp := decodeGantt(t, ganttGet(gateCtxFor(aliceCtx(), t, d), app, "plan?q=Delta"))
	if len(resp.Roots) != 0 {
		t.Errorf("LEAK: search found a hidden entity: %v", ganttIDs(resp))
	}
}
