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

// sidePanelAs drives handleV1SidePanel directly with a gated context.
// The form is always `ticketform`: these tests vary the PRINCIPAL and the
// entity, never the form.
const sidePanelFormID = "ticketform"

func sidePanelAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	entityID string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_sidepanel/"+sidePanelFormID+"/"+entityID, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.views.handleV1SidePanel(rec, req)
	return rec
}

// TestACLSidePanel_GatesHiddenEntity pins TKT-6N9O1Y: the side panel reveals the
// entry entity and its traversal neighbors, so it must respect the per-entity
// read gate. A principal who can read tickets gets the panel; one who cannot
// gets 404 — never the hidden entry's existence/data.
func TestACLSidePanel_GatesHiddenEntity(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "secret ticket"},
		Content:    "confidential body",
	})
	// A form with a (minimal) side panel so the handler reaches the gated read.
	app.Cfg().Forms["ticketform"] = dataentryconfig.Form{
		EntityType: "ticket",
		SidePanel:  &dataentryconfig.SidePanelConfig{},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// Visible: alice may read tickets → 200.
	rec := sidePanelAs(aliceCtx(), t, app, d, "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("alice _sidepanel: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Denied: bob has no role → DenyAll → 404, no leak.
	rec = sidePanelAs(principalCtx("bob"), t, app, d, "TKT-001")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob _sidepanel (denied): got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/errors/not_found") {
		t.Errorf("deny body missing not_found error code: %s", body)
	}
	if strings.Contains(body, "secret ticket") || strings.Contains(body, "confidential body") {
		t.Errorf("LEAK: denied _sidepanel exposed entity title/content: %s", body)
	}
}

// The side panel must apply the FACE half of a read grant, not just the row
// half (TKT-O7R2A1).
//
// It is the third caller of the shared view engine, and it reaches its entry
// through [entityReader.getEntity] — a RAW, ungated store read — after a
// gateRead that authorizes by (type, id) and so cannot see a face grant. That
// is the same shape as the leak fixed on the `_views` route, which served a
// draft body to a principal holding `policy@published`.
//
// Both directions are asserted: the denied face must 404, and the granted one
// must still render, or a blanket outage would pass for a working gate.
func TestACLSidePanel_GatesUngrantedFace(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	published := entity.Face("published")

	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "draft face"},
		Content:    "DRAFTONLY",
	})
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Face: published,
		Properties: map[string]any{"title": "published face"},
	}); err != nil {
		t.Fatalf("seed published face: %v", err)
	}

	app.Cfg().Forms["ticketform"] = dataentryconfig.Form{
		EntityType: "ticket",
		SidePanel:  &dataentryconfig.SidePanelConfig{},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket@published"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// The bare id addresses the DRAFT face, which this grant excludes.
	if rec := sidePanelAs(aliceCtx(), t, app, d, "TKT-001"); rec.Code != http.StatusNotFound {
		t.Errorf("side panel served the draft face to a principal granted only "+
			"ticket@published: got %d, want 404; body=%s", rec.Code, rec.Body)
	}

	// The granted face still renders.
	if rec := sidePanelAs(aliceCtx(), t, app, d, "TKT-001@published"); rec.Code != http.StatusOK {
		t.Errorf("the granted face must still render: got %d; body=%s", rec.Code, rec.Body)
	}
}
