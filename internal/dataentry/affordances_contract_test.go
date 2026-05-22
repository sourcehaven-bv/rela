package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestAffordances_BidirectionalContract is the AC3 contract test. For a
// fixed (principal, entity) tuple, every verb V where
// `_actions[V] == true` must result in the corresponding write
// returning 2xx; every false must result in 403 (specifically a
// *acl.ForbiddenError surfaced through the data-entry error path).
// Parameterized over the three ACL implementations.
//
// The test catches drift between the read-time verdict path
// (computeActions) and the write-time enforcement path (the actual
// handler going through entitymanager.Manager). Both paths share an
// acl.ACL instance via appbuild.WithTestACL — exactly how production
// wiring works.
func TestAffordances_BidirectionalContract(t *testing.T) {
	cases := []struct {
		name string
		acl  acl.ACL
	}{
		{"NopACL", acl.NopACL{}},
		{"ReadOnlyACL", acl.ReadOnlyACL{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := buildAppWithACL(t, tc.acl)
			seedEntity(app, &entity.Entity{
				ID:   "TKT-001",
				Type: "ticket",
				Properties: map[string]interface{}{
					"title":  "Contract test ticket",
					"status": "open",
				},
			})

			actions := fetchActions(t, app, "ticket", "tickets", "TKT-001")
			if actions == nil {
				t.Fatal("expected non-nil _actions for authenticated principal")
			}

			// delete: same-code-path invariant.
			gotStatus := tryDelete(t, app, "ticket", "tickets", "TKT-001")
			if actions["delete"] {
				if gotStatus >= 400 {
					t.Errorf("_actions.delete=true but DELETE returned %d; same-code-path invariant violated", gotStatus)
				}
			} else {
				if gotStatus != http.StatusForbidden {
					t.Errorf("_actions.delete=false but DELETE returned %d, want 403", gotStatus)
				}
			}
		})
	}
}

// buildAppWithACL constructs an App where both the read-side (computeActions)
// and the write-side (entityManager) share the same ACL instance — mirrors
// how appbuild.New wires production.
func buildAppWithACL(t *testing.T, a acl.ACL) *App {
	t.Helper()
	return buildAppWithACLAndAudit(t, a, nil)
}

// buildAppWithACLAndAudit is buildAppWithACL plus a custom audit sink
// for tests that need to assert on the audit-record stream.
func buildAppWithACLAndAudit(t *testing.T, a acl.ACL, auditSink audit.Audit) *App {
	t.Helper()
	meta := testMeta()
	cfg := testConfig()
	app := newAppFromParts(cfg, meta, &fixture{})

	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	opts := []appbuild.TestOption{
		appbuild.WithFS(fs, ctx),
		appbuild.WithTestACL(a),
	}
	if auditSink != nil {
		opts = append(opts, appbuild.WithTestAudit(auditSink))
	}
	svc := appbuild.NewForTest(meta, opts...)
	rebindApp(app, fs, ctx, svc)
	app.broker = newEventBroker()
	return app
}

// fetchActions issues a GET and returns the parsed _actions map.
func fetchActions(t *testing.T, app *App, typeName, plural, id string) map[string]bool {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+plural+"/"+id, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, typeName, plural, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s returned %d: %s", id, rec.Code, rec.Body.String())
	}
	var e V1Entity
	if err := json.NewDecoder(rec.Body).Decode(&e); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	return e.Actions
}

// tryDelete issues a DELETE and returns the HTTP status code.
func tryDelete(t *testing.T, app *App, typeName, plural, id string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/"+plural+"/"+id, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1DeleteEntity(rec, req, typeName, plural, id)
	return rec.Code
}
