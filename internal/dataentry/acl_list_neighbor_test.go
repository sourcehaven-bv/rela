package dataentry

import (
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestACLList_HiddenNeighborExcludedFromRelations is the regression pin for
// RR-HJV8CP: the per-row `relations` map must NOT carry the raw ID of a
// neighbor the caller cannot read — for BOTH an outgoing target and an incoming
// source. Before the fix, incomingRelations/outgoingRelations returned raw
// edges with no neighbor gating, so a hidden neighbor's ID landed in
// `relations[key]` while being absent from the visibility-filtered `included`
// map; the SPA then rendered the raw hidden ID in the cell.
//
// Visibility is per-instance via editor-of/belongs-to inheritance: entities
// reachable from PRJ-42 are visible to alice, entities reachable from PRJ-9 are
// hidden. The test asserts the hidden neighbor is dropped AND the visible
// neighbor is kept (the gate is not over-filtering).
func TestACLList_HiddenNeighborExcludedFromRelations(t *testing.T) {
	app := newTestAppV1(t)

	// ACL scaffolding: alice is editor-of PRJ-42; the editor role reads
	// tickets + features; the role is inherited through belongs-to. So
	// anything belonging to PRJ-42 is visible; anything belonging to PRJ-9 is
	// hidden.
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))

	// Row entities (visible) and their neighbors (mixed visibility).
	seedEntity(app, &entity.Entity{ID: "TKT-VIS", Type: "ticket", Properties: map[string]any{"title": "Visible ticket"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-VIS", Type: "feature", Properties: map[string]any{"title": "Visible feature"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-HIDDEN", Type: "feature", Properties: map[string]any{"title": "Hidden feature"}})
	seedEntity(app, &entity.Entity{ID: "TKT-HIDDEN", Type: "ticket", Properties: map[string]any{"title": "Hidden ticket"}})

	seedRelation(app, entity.NewRelation("TKT-VIS", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("FEAT-VIS", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("FEAT-HIDDEN", "belongs-to", "PRJ-9"))
	seedRelation(app, entity.NewRelation("TKT-HIDDEN", "belongs-to", "PRJ-9"))

	// Outgoing-target case (ticket row TKT-VIS): TKT-VIS implements a VISIBLE
	// and a HIDDEN feature. FEAT-HIDDEN must not leak into relations[implements].
	seedRelation(app, entity.NewRelation("TKT-VIS", "implements", "FEAT-HIDDEN"))

	// Incoming-source case (feature row FEAT-VIS): FEAT-VIS is implemented by a
	// VISIBLE ticket (TKT-VIS) and a HIDDEN ticket (TKT-HIDDEN). These land on
	// the feature row under the implements inverse key (implements has no
	// declared inverse -> synthetic implements_inverse). The TKT-VIS -> FEAT-VIS
	// edge is the visible incoming source; TKT-HIDDEN -> FEAT-VIS is the hidden
	// one that must not leak.
	seedRelation(app, entity.NewRelation("TKT-VIS", "implements", "FEAT-VIS"))
	seedRelation(app, entity.NewRelation("TKT-HIDDEN", "implements", "FEAT-VIS"))

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket", "feature"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d

	t.Run("outgoing target hidden is excluded, visible kept", func(t *testing.T) {
		resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET tickets: %d %s", rec.Code, rec.Body)
		}
		row := findRow(t, resp.Data, "TKT-VIS")
		impl := row.Relations["implements"]
		if containsID(impl, "FEAT-HIDDEN") {
			t.Errorf("relations[implements] leaks hidden target FEAT-HIDDEN: %v", impl)
		}
		if !containsID(impl, "FEAT-VIS") {
			t.Errorf("relations[implements] over-filtered visible target FEAT-VIS: %v", impl)
		}
	})

	t.Run("incoming source hidden is excluded, visible kept", func(t *testing.T) {
		resp, rec := listEntitiesAs(aliceCtx(), t, app, d, "feature", "features", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET features: %d %s", rec.Code, rec.Body)
		}
		row := findRow(t, resp.Data, "FEAT-VIS")
		// implements has no declared inverse -> synthetic <type>_inverse key.
		src := row.Relations["implements_inverse"]
		if containsID(src, "TKT-HIDDEN") {
			t.Errorf("relations[implements_inverse] leaks hidden source TKT-HIDDEN: %v", src)
		}
		if !containsID(src, "TKT-VIS") {
			t.Errorf("relations[implements_inverse] over-filtered visible source TKT-VIS: %v", src)
		}
	})
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
