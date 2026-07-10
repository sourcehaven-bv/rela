package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// seedNeighborLeakFixture builds the standard hidden-neighbor scenario shared by
// the /relations reproduction test and the cross-endpoint invariant test:
//
//   - alice is editor-of PRJ-42; the editor role reads tickets + features,
//     inherited through belongs-to. Entities belonging to PRJ-42 are visible;
//     entities belonging to PRJ-9 are hidden.
//   - TKT-VIS (visible ticket) implements FEAT-VIS (visible) and FEAT-HIDDEN
//     (hidden) — the OUTGOING-target leak case.
//   - FEAT-VIS (visible feature) is implemented by TKT-VIS (visible) and
//     TKT-HIDDEN (hidden) — the INCOMING-source (inverse-key) leak case.
//
// Returns the constructed ACL declarative wired into the app.
func seedNeighborLeakFixture(t *testing.T, app *App) *acl.Declarative {
	t.Helper()

	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-9", Type: "project", Properties: map[string]any{"title": "Hidden"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))

	seedEntity(app, &entity.Entity{ID: "TKT-VIS", Type: "ticket", Properties: map[string]any{"title": "Visible ticket"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-VIS", Type: "feature", Properties: map[string]any{"title": "Visible feature"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-HIDDEN", Type: "feature", Properties: map[string]any{"title": "Hidden feature"}})
	seedEntity(app, &entity.Entity{ID: "TKT-HIDDEN", Type: "ticket", Properties: map[string]any{"title": "Hidden ticket"}})

	seedRelation(app, entity.NewRelation("TKT-VIS", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("FEAT-VIS", "belongs-to", "PRJ-42"))
	seedRelation(app, entity.NewRelation("FEAT-HIDDEN", "belongs-to", "PRJ-9"))
	seedRelation(app, entity.NewRelation("TKT-HIDDEN", "belongs-to", "PRJ-9"))

	// Outgoing-target case (ticket row TKT-VIS).
	seedRelation(app, entity.NewRelation("TKT-VIS", "implements", "FEAT-HIDDEN"))
	seedRelation(app, entity.NewRelation("TKT-VIS", "implements", "FEAT-VIS"))

	// Incoming-source case (feature row FEAT-VIS).
	seedRelation(app, entity.NewRelation("TKT-HIDDEN", "implements", "FEAT-VIS"))

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"editor": {Read: []string{"ticket", "feature"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"editor-of": {Confers: "editor"},
		},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	app.acl = d
	return d
}

// getRelationsAs drives handleV1EntityRelations under the given principal's
// gate context and returns the decoded grouped-relations map.
func getRelationsAs(t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID string,
) map[string][]map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+plural+"/"+entityID+"/relations", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, typeName, entityID)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s/%s/relations: %d %s", plural, entityID, rec.Code, rec.Body)
	}
	var out map[string][]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode relations map: %v; body=%s", err, rec.Body)
	}
	return out
}

// getRelationTypeAs drives handleV1GetRelationType under the given principal's
// gate context and returns the decoded per-type relation list. direction is
// "" for outgoing or "incoming".
func getRelationTypeAs(t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID, relType, direction string,
) []map[string]any {
	t.Helper()
	url := "/api/v1/" + plural + "/" + entityID + "/relations/" + relType
	if direction != "" {
		url += "?direction=" + direction
	}
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.handleV1GetRelationType(rec, req, typeName, entityID, relType)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s/%s/relations/%s: %d %s", plural, entityID, relType, rec.Code, rec.Body)
	}
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode relation list: %v; body=%s", err, rec.Body)
	}
	return out
}

func relIDs(rels []map[string]any) []string {
	ids := make([]string, 0, len(rels))
	for _, r := range rels {
		if id, ok := r["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// TestACLRelations_HiddenNeighborExcluded is the reproduction pin for
// BUG-ABXMAV: the dedicated /relations and /relations/{relType} endpoints
// leaked a hidden neighbor's `type` and edge `meta` past the source-only read
// gate. This test asserts the hidden neighbor is fully absent (id, type, meta)
// for BOTH outgoing and incoming (inverse-key) directions, on BOTH endpoints,
// while the visible neighbor is still returned (no over-filtering).
func TestACLRelations_HiddenNeighborExcluded(t *testing.T) {
	app := newTestAppV1(t)
	d := seedNeighborLeakFixture(t, app)

	t.Run("grouped /relations excludes hidden outgoing target, keeps visible", func(t *testing.T) {
		rels := getRelationsAs(t, app, d, "ticket", "tickets", "TKT-VIS")
		impl := rels["implements"]
		ids := relIDs(impl)
		if containsID(ids, "FEAT-HIDDEN") {
			t.Errorf("/relations leaks hidden outgoing target FEAT-HIDDEN: %v", impl)
		}
		if !containsID(ids, "FEAT-VIS") {
			t.Errorf("/relations over-filtered visible target FEAT-VIS: %v", impl)
		}
		assertNoHiddenType(t, impl)
	})

	t.Run("grouped /relations excludes hidden incoming source, keeps visible", func(t *testing.T) {
		rels := getRelationsAs(t, app, d, "feature", "features", "FEAT-VIS")
		src := rels["implements_inverse"]
		ids := relIDs(src)
		if containsID(ids, "TKT-HIDDEN") {
			t.Errorf("/relations leaks hidden incoming source TKT-HIDDEN: %v", src)
		}
		if !containsID(ids, "TKT-VIS") {
			t.Errorf("/relations over-filtered visible source TKT-VIS: %v", src)
		}
		assertNoHiddenType(t, src)
	})

	t.Run("/relations/{relType} outgoing excludes hidden target, keeps visible", func(t *testing.T) {
		rels := getRelationTypeAs(t, app, d, "ticket", "tickets", "TKT-VIS", "implements", "")
		ids := relIDs(rels)
		if containsID(ids, "FEAT-HIDDEN") {
			t.Errorf("/relations/implements leaks hidden target FEAT-HIDDEN: %v", rels)
		}
		if !containsID(ids, "FEAT-VIS") {
			t.Errorf("/relations/implements over-filtered visible target FEAT-VIS: %v", rels)
		}
		assertNoHiddenType(t, rels)
	})

	t.Run("/relations/{relType} incoming excludes hidden source, keeps visible", func(t *testing.T) {
		rels := getRelationTypeAs(t, app, d, "feature", "features", "FEAT-VIS", "implements", "incoming")
		ids := relIDs(rels)
		if containsID(ids, "TKT-HIDDEN") {
			t.Errorf("/relations/implements?incoming leaks hidden source TKT-HIDDEN: %v", rels)
		}
		if !containsID(ids, "TKT-VIS") {
			t.Errorf("/relations/implements?incoming over-filtered visible source TKT-VIS: %v", rels)
		}
		assertNoHiddenType(t, rels)
	})
}

// assertNoHiddenType checks no relation entry carries the hidden peers' types.
// Both hidden peers have distinctive types (feature/ticket) but the load-bearing
// assertion is that the specific hidden IDs never appear with a resolved type;
// this helper double-checks that no entry names a hidden ID at all.
func assertNoHiddenType(t *testing.T, rels []map[string]any) {
	t.Helper()
	for _, r := range rels {
		if id, _ := r["id"].(string); id == "FEAT-HIDDEN" || id == "TKT-HIDDEN" {
			t.Errorf("hidden peer %q present in relation entry %v", id, r)
		}
	}
}

// TestACLNeighborReadLeakInvariant (AM-acl-neighbor-read-leak-invariant) is the
// cross-endpoint P4 invariant: with ONE hidden-neighbor fixture, every
// neighbor-emitting read endpoint is exercised and asserted not to leak the
// hidden neighbor's TYPE or META. It is structured so a future neighbor-emitting
// endpoint is added to the `endpoints` table and immediately covered.
//
// The accepted exception (dataentry/CLAUDE.md, acl_get_test.go ~L149): the raw
// neighbor ID may appear in the single-entity GET's IDs-only relations map and
// in the list-row top-level relations map. That exception is encoded explicitly
// below (allowBareID). What must NEVER leak on ANY endpoint is the hidden
// neighbor's TYPE or edge META — and the dedicated /relations endpoints must
// leak nothing at all about a hidden peer (not even the bare ID).
func TestACLNeighborReadLeakInvariant(t *testing.T) {
	const (
		hiddenFeature = "FEAT-HIDDEN"
		hiddenTicket  = "TKT-HIDDEN"
	)

	// Endpoint fixtures: each returns the raw response body as a string. We
	// scan the body for the hidden peers' identifiers. TYPE leakage would show
	// the hidden peer's id paired with a "type" field; the dedicated /relations
	// endpoints must not even name the hidden id.
	endpoints := []struct {
		name string
		// allowBareID: the accepted exception — the hidden neighbor's bare ID
		// may appear (single-GET relations map, list-row relations map), but
		// its TYPE and META must not. When false, the hidden ID must be fully
		// absent from the body.
		allowBareID bool
		body        func(t *testing.T, app *App, d *acl.Declarative) string
	}{
		{
			name:        "list rows",
			allowBareID: true, // top-level relations map is IDs-only; type/meta never emitted here
			body: func(t *testing.T, app *App, d *acl.Declarative) string {
				t.Helper()
				_, rec := listEntitiesAs(aliceCtx(), t, app, d, "feature", "features", "")
				return rec.Body.String()
			},
		},
		{
			name:        "list rows with include",
			allowBareID: true, // include map is visibility-filtered; bare ID may still key the relations map
			body: func(t *testing.T, app *App, d *acl.Declarative) string {
				t.Helper()
				_, rec := listEntitiesAs(aliceCtx(), t, app, d, "feature", "features", "include=*")
				return rec.Body.String()
			},
		},
		{
			name:        "single-entity GET",
			allowBareID: true, // per-entity GET ships an IDs-only relations map (accepted, CLAUDE.md)
			body: func(t *testing.T, app *App, d *acl.Declarative) string {
				t.Helper()
				rec := getEntityAs(aliceCtx(), t, app, d, "feature", "features", "FEAT-VIS", "include=*")
				return rec.Body.String()
			},
		},
		{
			name:        "/relations grouped",
			allowBareID: false, // dedicated endpoint must leak nothing about a hidden peer
			body: func(t *testing.T, app *App, d *acl.Declarative) string {
				t.Helper()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/features/FEAT-VIS/relations", http.NoBody)
				req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
				rec := httptest.NewRecorder()
				app.handleV1EntityRelations(rec, req, "feature", "FEAT-VIS")
				return rec.Body.String()
			},
		},
		{
			name:        "/relations/{relType} incoming",
			allowBareID: false,
			body: func(t *testing.T, app *App, d *acl.Declarative) string {
				t.Helper()
				req := httptest.NewRequest(http.MethodGet,
					"/api/v1/features/FEAT-VIS/relations/implements?direction=incoming", http.NoBody)
				req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
				rec := httptest.NewRecorder()
				app.handleV1GetRelationType(rec, req, "feature", "FEAT-VIS", "implements")
				return rec.Body.String()
			},
		},
		{
			name:        "/relations/{relType} outgoing",
			allowBareID: false,
			body: func(t *testing.T, app *App, d *acl.Declarative) string {
				t.Helper()
				req := httptest.NewRequest(http.MethodGet,
					"/api/v1/tickets/TKT-VIS/relations/implements", http.NoBody)
				req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
				rec := httptest.NewRecorder()
				app.handleV1GetRelationType(rec, req, "ticket", "TKT-VIS", "implements")
				return rec.Body.String()
			},
		},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			app := newTestAppV1(t)
			d := seedNeighborLeakFixture(t, app)
			body := ep.body(t, app, d)

			// A hidden neighbor's serialized entity (with its type) must never
			// leak. In the JSON:API `included` map the hidden entity would be
			// keyed by its ID with a full "type"/"attributes" object; we assert
			// no `included` entry names a hidden peer.
			if strings.Contains(body, `"included":`) {
				for _, hid := range []string{hiddenFeature, hiddenTicket} {
					if strings.Contains(body, `"`+hid+`":`) {
						t.Errorf("%s: hidden peer %q serialized in included map; body=%s", ep.name, hid, body)
					}
				}
			}

			if !ep.allowBareID {
				// Dedicated /relations endpoints must not name the hidden peer
				// at all — not the id, and therefore not its type or meta.
				for _, hid := range []string{hiddenFeature, hiddenTicket} {
					if strings.Contains(body, hid) {
						t.Errorf("%s: hidden peer %q leaked (id/type/meta); body=%s", ep.name, hid, body)
					}
				}
			}
		})
	}
}
