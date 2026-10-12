package dataentry

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// singleMembershipApp is a dynamic-collection app whose belongs-to allows a
// task one project.
func singleMembershipApp(t *testing.T, ents []*entity.Entity, rels []*entity.Relation) (*App, *caldavBackend) {
	t.Helper()
	one := 1
	app := caldavDynamicAppWith(t, ents, rels)
	meta := app.State().Meta
	def := meta.Relations["belongs-to"]
	def.MaxOutgoing = &one
	def.MinOutgoing = &one // a last unlink would apply on_delete
	meta.Relations["belongs-to"] = def
	return app, &caldavBackend{app: app, baseURL: "https://example.test"}
}

// projectsOf lists the projects id belongs to, sorted.
func projectsOf(t *testing.T, app *App, id string) []string {
	t.Helper()
	var out []string
	for r, err := range app.Services().Store.ListRelations(t.Context(),
		store.RelationQuery{From: id, Type: "belongs-to"}) {
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		out = append(out, r.To)
	}
	slices.Sort(out)
	return out
}

// An edit of a to-do in a collection the client already holds it in only
// re-asserts the membership. After a move to PRJ-2, a stale edit from PRJ-1
// must not move the to-do back: the bound refuses the edge, and the edit is
// answered as a refused write.
func TestDynamicCollections_StaleEditDoesNotUndoMove(t *testing.T) {
	app, b := singleMembershipApp(t,
		[]*entity.Entity{mkProject("PRJ-1", "Alpha"), mkProject("PRJ-2", "Beta"), mkTaskIn("TSK-A", "work")},
		[]*entity.Relation{entity.NewRelation("TSK-A", "belongs-to", "PRJ-1")})
	href := "task--TSK-A@rela.ics"
	put := func(collection, title string) error {
		_, err := b.PutCalendarObject(t.Context(), b.calendarPath(collection)+href,
			mustParseICal(t, caldavDynamicBody("task--TSK-A@rela", title)), nil)
		return err
	}
	// The client writes the to-do in PRJ-1 once, so it holds an alias there.
	if err := put("project_tasks--PRJ-1", "work"); err != nil {
		t.Fatalf("edit in PRJ-1: %v", err)
	}
	if err := put("project_tasks--PRJ-2", "work"); err != nil {
		t.Fatalf("move into PRJ-2: %v", err)
	}
	if got := projectsOf(t, app, "TSK-A"); !slices.Equal(got, []string{"PRJ-2"}) {
		t.Fatalf("memberships after move = %v, want [PRJ-2]", got)
	}
	if err := put("project_tasks--PRJ-1", "stale"); err != nil {
		t.Fatalf("stale edit in PRJ-1: %v, want the refused-write answer", err)
	}
	if got := projectsOf(t, app, "TSK-A"); !slices.Equal(got, []string{"PRJ-2"}) {
		t.Errorf("memberships after stale edit = %v, want [PRJ-2]", got)
	}
}

// A DELETE whose membership edge is already gone succeeds only for a readable
// entity with another membership. A hidden entity and one that belongs to no
// project both get the opaque 404, so the answer does not reveal which ids
// exist.
func TestDynamicCollections_MissingMembershipDeleteIsNotAnOracle(t *testing.T) {
	_, b := singleMembershipApp(t,
		[]*entity.Entity{
			mkProject("PRJ-1", "Alpha"), mkProject("PRJ-2", "Beta"),
			mkTaskIn("TSK-H", "hidden"), mkTaskIn("TSK-N", "no project"),
		},
		[]*entity.Relation{entity.NewRelation("TSK-H", "belongs-to", "PRJ-2")})
	ctx := withReadGate(t.Context(), configGate{permits: map[string]bool{
		"PRJ-1": true, "PRJ-2": true, "TSK-N": true,
	}})
	for _, id := range []string{"TSK-H", "TSK-N", "TSK-NONE"} {
		err := b.DeleteCalendarObject(ctx, b.calendarPath("project_tasks--PRJ-1")+"task--"+id+"@rela.ics")
		if err == nil || !strings.HasPrefix(err.Error(), "404 ") {
			t.Errorf("DELETE %s = %v, want a 404", id, err)
		}
	}
}

// TestDynamicCollections_MoveOnSingleMembership pins a client move into
// another collection when the membership relation allows one edge
// (max_outgoing: 1, TKT-65LVAK). The client PUTs the to-do into the new
// collection, then DELETEs it from the old one. The PUT must re-point the
// edge instead of being refused by the bound, and the DELETE must then
// succeed without disposing of the to-do as if it had lost its last
// membership.
func TestDynamicCollections_MoveOnSingleMembership(t *testing.T) {
	one := 1
	app := caldavDynamicAppWith(t,
		[]*entity.Entity{mkProject("PRJ-1", "Alpha"), mkProject("PRJ-2", "Beta"), mkTaskIn("TSK-A", "work")},
		[]*entity.Relation{entity.NewRelation("TSK-A", "belongs-to", "PRJ-1")})
	meta := app.State().Meta
	def := meta.Relations["belongs-to"]
	def.MaxOutgoing = &one
	def.MinOutgoing = &one // a last unlink would apply on_delete
	meta.Relations["belongs-to"] = def
	b := &caldavBackend{app: app, baseURL: "https://example.test"}

	memberships := func() []string {
		t.Helper()
		var out []string
		for r, err := range app.Services().Store.ListRelations(t.Context(),
			store.RelationQuery{From: "TSK-A", Type: "belongs-to"}) {
			if err != nil {
				t.Fatalf("ListRelations: %v", err)
			}
			out = append(out, r.To)
		}
		slices.Sort(out)
		return out
	}

	if _, err := b.PutCalendarObject(t.Context(),
		b.calendarPath("project_tasks--PRJ-2")+"task--TSK-A@rela.ics",
		mustParseICal(t, caldavDynamicBody("task--TSK-A@rela", "work")), nil); err != nil {
		t.Fatalf("PUT into the new collection: %v", err)
	}
	if got := memberships(); !slices.Equal(got, []string{"PRJ-2"}) {
		t.Fatalf("memberships after PUT = %v, want [PRJ-2]", got)
	}

	if err := b.DeleteCalendarObject(t.Context(),
		b.calendarPath("project_tasks--PRJ-1")+"task--TSK-A@rela.ics"); err != nil {
		t.Fatalf("DELETE from the old collection: %v", err)
	}
	if got := memberships(); !slices.Equal(got, []string{"PRJ-2"}) {
		t.Errorf("memberships after DELETE = %v, want [PRJ-2]", got)
	}
	e, err := app.Services().Store.GetEntity(t.Context(), entity.Ref{ID: "TSK-A"})
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got := e.GetString("status"); got != "todo" {
		t.Errorf("status = %q, want todo: the DELETE disposed of a to-do that is still a member", got)
	}
}
