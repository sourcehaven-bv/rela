package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// viewsAs drives handleV1Views directly with a gated context, mirroring
// getEntityAs (the GET-entity equivalent). The middleware is bypassed in
// unit tests, so gateCtxFor attaches the readGate the handler reads.
//
// entityType is kept explicit (mirroring getEntityAs and the route shape) so a
// future non-ticket case needs no signature churn.
//
//nolint:unparam // see above: entityType is intentionally parameterized.
func viewsAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	entityType, entityID string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_views/"+entityType+"/"+entityID, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.views.handleV1Views(rec, req)
	return rec
}

// TestACLViews_GatesHiddenEntity pins TKT-BNX2PN: _views is an entity-read
// chokepoint and MUST respect the per-entity read gate. A principal who can
// read tickets sees the view; one who cannot gets 404 with the same not_found
// shape as a missing id — never the title or content body of a hidden entity.
func TestACLViews_GatesHiddenEntity(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "secret ticket"},
		Content:    "confidential body",
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// Visible: alice may read tickets.
	rec := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("alice _views: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Denied: bob holds no role → DenyAll on ticket. Must 404, and the body
	// must NOT leak the title or content body.
	rec = viewsAs(principalCtx("bob"), t, app, d, "ticket", "TKT-001")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob _views (denied): got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "/errors/not_found") {
		t.Errorf("deny body missing not_found error code: %s", rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "secret ticket") || strings.Contains(body, "confidential body") {
		t.Errorf("LEAK: denied _views response exposed entity title/content: %s", body)
	}

	// A denied id is indistinguishable from a hidden one (404 not_found); the
	// gate runs BEFORE executeView so the denied principal never reaches the
	// view pipeline (which would otherwise 422 on a missing entity and leak a
	// different shape). For an *allowed* type the existing missing-entity path
	// (executeView → 422) is unchanged and out of scope for this gate.
}

// TestACLViews_RedactsHiddenPropertyValue pins BUG-9QL9XV: a VISIBLE entity's
// view must not leak a field-level `visible:`-hidden property VALUE. The entity
// endpoints already strip via the serializer, but the _views section payload
// built its fields[].values straight from raw e.Properties, bypassing the
// strip. Routing executeView through the view reader (visibility.PolicyReader)
// closes it: section builders now receive already-redacted entities.
func TestACLViews_RedactsHiddenPropertyValue(t *testing.T) {
	app := newTestAppV1(t)
	// Hide `status` for everyone; title stays visible so the entity is readable
	// and the view renders. `status` is a rendered field in the default ticket
	// view, so its value flows into a section's fields[].values — the exact
	// surface that leaked. (Hiding a property the default view does NOT render
	// would make the test vacuous.)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"status": false},
	}}
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "visible title", "status": "SECRET-STATUS"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	rec := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("alice _views: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "SECRET-STATUS") {
		t.Errorf("LEAK: hidden property value appeared in _views response: %s", body)
	}
}

// TestACLViews_RedactsHiddenPrimaryTitle pins BUG-R9EHKV: when the display
// (primary) property itself is hidden, the section entity title must fall back
// to the id rather than leaking the hidden value. visibility.Redact recomputes
// the title on redaction; before the fix, sections.go called DisplayTitle
// against raw properties and emitted the hidden value as the section title.
func TestACLViews_RedactsHiddenPrimaryTitle(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}}
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "SECRET-TITLE"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	rec := viewsAs(aliceCtx(), t, app, d, "ticket", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("alice _views: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "SECRET-TITLE") {
		t.Errorf("LEAK: hidden primary property leaked as a section title: %s", body)
	}
}

// TestACLViews_RelationColumnRedactsHiddenNeighborTitle pins the review finding
// on BUG-R9EHKV's table-cell surface: a table section's relation column resolves
// neighbor titles via resolveRelationColumnValues, which fetched the target raw
// from the store and titled it against raw properties — leaking a hidden
// neighbor's display value. The fix routes the targets through viewReader.Filter
// (row-gate + redact), so a hidden primary falls back to the id. This surface is
// NOT covered by the entry-only tests above (it fetches targets fresh, not from
// result.Collections).
func TestACLViews_RelationColumnRedactsHiddenNeighborTitle(t *testing.T) {
	app := newTestAppV1(t)
	// Hide `title` everywhere; feature's only property is its display title, so a
	// followed `implements` edge would otherwise leak the feature's title.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}}
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "ticket title"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "SECRET-FEATURE"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-001"))

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	titles := app.views.resolveRelationColumnValues(gateCtxFor(aliceCtx(), t, d), "TKT-001", "implements", dataentryconfig.DirectionOutgoing)
	for _, tt := range titles {
		if strings.Contains(tt, "SECRET-FEATURE") {
			t.Errorf("LEAK: hidden neighbor display value in relation column: %v", titles)
		}
	}
}

// TestACLViews_RelationColumnDropsUnreadableNeighbor pins the row-gate half of
// the same fix: a neighbor the principal may NOT read must not appear as a
// column value at all (not just have its title redacted). Before the fix the
// raw store read ignored the read gate entirely.
func TestACLViews_RelationColumnDropsUnreadableNeighbor(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "ticket title"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "unreadable feature"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-001"))

	// viewer reads tickets but NOT features → the neighbor is unreadable.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	titles := app.views.resolveRelationColumnValues(gateCtxFor(aliceCtx(), t, d), "TKT-001", "implements", dataentryconfig.DirectionOutgoing)
	if len(titles) != 0 {
		t.Errorf("unreadable neighbor leaked into relation column: %v", titles)
	}
}

// TestACLScript_RedactedVisibleToLua is the end-to-end verification for
// TKT-FJ6END on the data-entry path — the one production wiring that has
// a real field resolver (the scheduler's does not; see RR-7408F5). It runs
// a script through App.scriptReader exactly as document rendering does.
func TestACLScript_RedactedVisibleToLua(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"status": false},
	}}
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "visible title", "status": "SECRET-STATUS"},
	})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	reader := app.scriptReader(appRedactor(app))
	e, err := reader.GetEntity(aliceCtx(), "TKT-001")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}

	if got, ok := e.Properties["status"]; ok {
		t.Errorf("LEAK: hidden value still present: %v", got)
	}
	if !e.IsRedacted("status") {
		t.Errorf("hidden property not marked redacted; Redacted=%v", e.Redacted)
	}
	if e.IsRedacted("title") {
		t.Errorf("granted property wrongly marked redacted; Redacted=%v", e.Redacted)
	}
	if e.IsLocked() {
		t.Error("redacted entity reports IsLocked; write paths and the validator would skip it")
	}
}
