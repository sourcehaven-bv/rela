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

// documentsAs drives handleV1Documents directly with a gated context.
func documentsAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	docName, entityID string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_documents/"+docName+"/"+entityID, http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, req)
	return rec
}

// TestACLDocuments_GatesHiddenEntity pins TKT-C0R07J: document rendering serves
// entity-derived content (and may run a Lua script reading related entities), so
// it must respect the per-entity read gate. A denied principal gets a 404 BEFORE
// the renderer runs — never the rendered document, and not even a type-mismatch
// 400 oracle.
func TestACLDocuments_GatesHiddenEntity(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "secret ticket"},
		Content:    "confidential body",
	})
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"ticket-summary": {EntityType: "ticket", Command: "echo LEAKED"},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// Denied: bob has no role → 404, and the renderer (echo LEAKED) must NOT run.
	rec := documentsAs(principalCtx("bob"), t, app, d, "ticket-summary", "TKT-001")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob _documents (denied): got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/errors/not_found") {
		t.Errorf("deny body missing not_found error code: %s", body)
	}
	if strings.Contains(body, "LEAKED") || strings.Contains(body, "secret ticket") {
		t.Errorf("LEAK: denied _documents rendered content or ran the renderer: %s", body)
	}

	// Permitted: alice may read tickets, so the gate passes and the handler
	// proceeds past it (render may succeed or fail, but it is NOT our 404).
	rec = documentsAs(aliceCtx(), t, app, d, "ticket-summary", "TKT-001")
	if rec.Code == http.StatusNotFound {
		t.Fatalf("alice _documents: got 404, want the gate to pass; body=%s", rec.Body)
	}
}
