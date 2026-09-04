package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A `read: [type@face]` grant must gate the LIST and the single-entity GET
// identically (TKT-O7R2A1).
//
// Both halves are load-bearing and both were wrong at some point while this
// was built: a first cut gated only the GET, and a second gated only the
// pushdown seam while the HTTP list path — a SECOND composition site with its
// own copy of the same logic — kept serving denied faces. Neither is caught by
// testing one path.
//
// The principal takes the AllowAll branch here deliberately. That is the half a
// parity test most easily misses (see TestWorldListGetParity_ACLGatedPrincipal
// for the conferred/GraphQuery half), and it is where a face filter carried
// only on GraphQuery would leave the most privileged callers ungated.
func TestFaceGrantParity_ListAndGetAgree(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()

	// TKT-1 has both a default (draft) face and a published one.
	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket",
		Properties: map[string]any{"title": "draft face"},
	})
	published := entity.Face("published")
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Face: published,
		Properties: map[string]any{"title": "published face"},
	}); err != nil {
		t.Fatalf("seed published state: %v", err)
	}

	// The grant names ONE face. A global assignment, so ReadQuery returns
	// AllowAll and the list takes its EntityQuery branch.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket@published"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	req, rerr := d.ForPrincipal(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	if rerr != nil {
		t.Fatalf("ForPrincipal: %v", rerr)
	}
	gate, gerr := newACLReadGate(req)
	if gerr != nil {
		t.Fatalf("newACLReadGate: %v", gerr)
	}
	gctx := withReadGate(aliceCtx(), gate)

	// Precondition: this must be the AllowAll branch, or the test silently
	// exercises the other half and proves nothing about this one.
	if rqr := gate.ReadQuery(gctx, "ticket"); !rqr.AllowAll {
		t.Fatalf("want the AllowAll branch, got %+v", rqr)
	}

	// LIST: only the granted face may appear.
	var listed []*entity.Entity
	for e, err := range app.store.ListEntities(gctx, store.EntityQuery{
		Type:      "ticket",
		AllStates: true,
		FaceIn:    gate.ReadQuery(gctx, "ticket").Faces,
	}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		listed = append(listed, e)
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %d rows, want 1 (the published face only): %+v", len(listed), listed)
	}
	if listed[0].Face != published {
		t.Errorf("list returned face %q, want published — the denied draft leaked", listed[0].Face)
	}

	// GET: the denied face is indistinguishable from a miss, and the granted
	// one is served.
	_, foundDraft, err := app.visibleReader.getVisible(gctx, "ticket", "TKT-1")
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if foundDraft {
		t.Error("GET returned the DRAFT face, which the grant does not name")
	}
}

// A bare type grant reads every face — the backward-compatible default.
//
// Not merely convenient: a world resolves each entity through its chain and
// never serves the default face, so a bare grant narrowed to the default would
// read NOTHING under any world. That is a total outage rather than a narrowing,
// which is why reads differ from writes here.
func TestFaceGrant_BareGrantReadsEveryFace(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()

	seedEntity(app, &entity.Entity{
		ID: "TKT-2", Type: "ticket",
		Properties: map[string]any{"title": "draft"},
	})
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-2", Type: "ticket", Face: entity.Face("published"),
		Properties: map[string]any{"title": "published"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	req, _ := d.ForPrincipal(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	gate, gerr := newACLReadGate(req)
	if gerr != nil {
		t.Fatalf("newACLReadGate: %v", gerr)
	}
	gctx := withReadGate(aliceCtx(), gate)

	if faces := gate.ReadQuery(gctx, "ticket").Faces; faces != nil {
		t.Fatalf("a bare grant must push NO face filter (nil = every face), got %v", faces)
	}
	if _, found, err := app.visibleReader.getVisible(gctx, "ticket", "TKT-2"); err != nil || !found {
		t.Errorf("a bare grant must still read the entity: found=%v err=%v", found, err)
	}
}

// The VIEW surface owes an entity the same face gate the entity route applies.
//
// It did not, and that was a cross-principal content leak: a principal granted
// `ticket@published` got a 200 from `_views/ticket/TKT-1` carrying the DRAFT
// body, while `GET /tickets/TKT-1` 404'd the very same face. Found by QA
// probing the two routes against one fixture.
//
// The cause was structural rather than a missed line. viewsHandler holds BOTH a
// raw store.Store and a visibility.Reader, and executeView resolved its ENTRY
// through the raw handle — so the row gate ran one layer up (by type and id,
// which a face grant is invisible to) and nothing checked the face. Because
// executeView is a shared engine, `_views`, `_sidepanel` and the command
// runner's `kind: view` all inherited it.
//
// Both worlds are asserted: the default-world branch is where the leak was, and
// the world branch is where "a world is a view, never a permission" has to hold
// — a world that resolves TO a denied face must still be refused.
func TestFaceGrant_ViewEntryIsFaceGated(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	seedViewGraph(t, app)

	// Precondition: ungated, the default world serves the draft entry. Without
	// this the assertions below could pass against an empty view.
	pre, err := app.views.executeView(ctx, implementsView(), "TKT-1", defaultViewWorld())
	if err != nil {
		t.Fatalf("ungated default world: %v", err)
	}
	if got, _ := pre.Entry.Properties["title"].(string); got != "draft entry" {
		t.Fatalf("precondition: want the draft entry ungated, got %q", got)
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket@published"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	req, rerr := d.ForPrincipal(principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	if rerr != nil {
		t.Fatalf("ForPrincipal: %v", rerr)
	}
	gate, gerr := newACLReadGate(req)
	if gerr != nil {
		t.Fatalf("newACLReadGate: %v", gerr)
	}
	gctx := withReadGate(aliceCtx(), gate)

	// The DEFAULT world resolves the draft face, which this grant excludes.
	if res, viewErr := app.views.executeView(
		gctx, implementsView(), "TKT-1", defaultViewWorld()); viewErr == nil {
		title, _ := res.Entry.Properties["title"].(string)
		t.Errorf("view served face %q (title %q) to a principal granted only "+
			"ticket@published — the draft leaked", res.Entry.Face, title)
	}

	// The granted face is still served, or the gate above is just an outage.
	pub, err := app.views.executeView(
		gctx, implementsView(), "TKT-1", publishedViewWorld("ticket", "feature"))
	if err != nil {
		t.Fatalf("the granted face must still render: %v", err)
	}
	if got, _ := pub.Entry.Properties["title"].(string); got != "published entry" {
		t.Errorf("want the published entry, got %q", got)
	}
}

// The relation read surfaces owe the entity a face check too.
//
// They carry no body, which is why this was easy to miss — but a
// content-scoped relation belongs to ONE face, so serving the default face's
// edges to a principal granted only `ticket@published` discloses the structure
// of a face the grant withholds: the edge's existence, its type, and the
// neighbor's id.
//
// Found by QA against a live server. The first probe was VACUOUS — the fixture
// had no relations, so both the granted and denied principals got `{}` and the
// surface looked clean. It only failed once real edges existed.
func TestFaceGrant_RelationReadsAreFaceGated(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	published := entity.Face("published")

	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft face"},
	})
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Face: published,
		Properties: map[string]any{"title": "published face"},
	}); err != nil {
		t.Fatalf("seed published face: %v", err)
	}
	seedEntity(app, &entity.Entity{
		ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "neighbor"},
	})
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: []string{"ticket@published", "feature"}},
		},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// Precondition: a principal who may read the draft face DOES see the edge,
	// so a 404 below means the face gate fired rather than the fixture being
	// empty — the exact vacuity that hid this in manual testing.
	full := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"*"}}},
		Assignments: map[string]string{"bob": "admin"},
	}, app.store)
	app.acl = full
	rec := relationsAs(principalCtx("bob"), t, app, full, "ticket", "tickets", "TKT-1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "FEAT-1") {
		t.Fatalf("precondition: an unrestricted principal must see the edge; got %d body=%s",
			rec.Code, rec.Body)
	}

	app.acl = d
	rec = relationsAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-1")
	if rec.Code != http.StatusNotFound {
		t.Errorf("relations of the DRAFT face served to a principal granted only "+
			"ticket@published: got %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// relationsAs drives handleV1EntityRelations with a gated context.
func relationsAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	typeName, plural, entityID string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/"+plural+"/"+entityID+"/relations", http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1EntityRelations(rec, req, typeName, entityID)
	return rec
}
