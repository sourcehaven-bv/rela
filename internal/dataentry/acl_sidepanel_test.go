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
func sidePanelAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	formID, entityID string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_sidepanel/"+formID+"/"+entityID, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1SidePanel(rec, req)
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
	rec := sidePanelAs(aliceCtx(), t, app, d, "ticketform", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("alice _sidepanel: got %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// Denied: bob has no role → DenyAll → 404, no leak.
	rec = sidePanelAs(principalCtx("bob"), t, app, d, "ticketform", "TKT-001")
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
