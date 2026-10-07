package dataentry

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

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
