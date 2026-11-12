package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// deleteACL is updatedACL with delete on tickets: alice sees only the title
// and the reason, bob sees everything.
const deleteACL = `
roles:
  partial:
    read: [ticket]
    update: [ticket]
    delete: [ticket]
    visible:
      ticket:
        - field: title
    relations:
      ticket:
        - relation: depends_on
          visible:
            - field: reason
  full:
    read: [ticket]
    update: [ticket]
    delete: [ticket]
  outsider: {}
assignments:
  alice: partial
  bob: full
  carol: outsider
`

func deleteApp(t *testing.T) *App {
	t.Helper()
	app := buildPolicyApp(t, deleteACL, nil)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "a", "status": "open"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	if _, err := app.store.CreateRelation(context.Background(),
		entity.RelationKey{From: "TKT-001", Type: "depends_on", To: "TKT-002"},
		&store.RelationData{Properties: map[string]any{"reason": "r", "secret": "s"}}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	return app
}

func getEntityETag(t *testing.T, app *App, user string) string {
	t.Helper()
	req := boundAs(t, app, httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody), user)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "ticket", "tickets", "TKT-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	return rec.Header().Get("ETag")
}

// gatedAs binds user and attaches the row gate the request middleware
// attaches in production, so a row the user cannot read is missing.
func gatedAs(t *testing.T, app *App, req *http.Request, user string) *http.Request {
	t.Helper()
	d, ok := app.acl.(*acl.Declarative)
	if !ok {
		t.Fatalf("app ACL is %T, want a policy", app.acl)
	}
	ctx := principal.With(req.Context(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
	return req.WithContext(gateCtxFor(ctx, t, d))
}

func deleteEntity(t *testing.T, app *App, user, ifMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tickets/TKT-001", http.NoBody)
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rec := httptest.NewRecorder()
	app.write.handleV1DeleteEntity(rec, gatedAs(t, app, req, user), "ticket", "tickets", "TKT-001")
	return rec
}

func deleteRelation(t *testing.T, app *App, user, ifMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/tickets/TKT-001/relations/depends_on/TKT-002", http.NoBody)
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rec := httptest.NewRecorder()
	app.write.handleV1DeleteRelation(rec, gatedAs(t, app, req, user), "ticket", "TKT-001", "depends_on", "TKT-002")
	return rec
}

// A DELETE with If-Match deletes only the row the caller last saw: a stale
// ETag is a 412 and leaves the row, the GET's ETag deletes it, and no
// If-Match deletes unconditionally (TKT-CVT9W1K).
func TestDeleteEntity_IfMatch(t *testing.T) {
	for _, tc := range []struct{ name, ifMatch string }{{"matching", "get"}, {"absent", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			app := deleteApp(t)
			etag := getEntityETag(t, app, "bob")
			if rec := deleteEntity(t, app, "bob", `"stale"`); rec.Code != http.StatusPreconditionFailed {
				t.Fatalf("DELETE with a stale ETag = %d %s, want 412", rec.Code, rec.Body)
			}
			if getEntityETag(t, app, "bob") != etag {
				t.Fatal("a refused DELETE changed the row")
			}
			ifMatch := tc.ifMatch
			if ifMatch == "get" {
				ifMatch = etag
			}
			if rec := deleteEntity(t, app, "bob", ifMatch); rec.Code != http.StatusNoContent {
				t.Fatalf("DELETE = %d %s, want 204", rec.Code, rec.Body)
			}
		})
	}
}

// The compare is against the caller's view: a change to a field alice cannot
// see does not move her ETag, so her delete still applies and a 412 cannot
// tell her a hidden field changed.
func TestDeleteEntity_IfMatchIgnoresHiddenFields(t *testing.T) {
	app := deleteApp(t)
	etag := getEntityETag(t, app, "alice")
	if err := app.store.UpdateEntity(context.Background(), &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "a", "status": "closed"}}); err != nil {
		t.Fatalf("UpdateEntity: %v", err)
	}
	if rec := deleteEntity(t, app, "alice", etag); rec.Code != http.StatusNoContent {
		t.Fatalf("alice DELETE after a hidden change = %d %s, want 204", rec.Code, rec.Body)
	}
}

func TestDeleteRelation_IfMatch(t *testing.T) {
	for _, user := range []string{"bob", "alice"} {
		t.Run(user, func(t *testing.T) {
			app := deleteApp(t)
			etag := getRelationETag(t, app, user)
			if rec := deleteRelation(t, app, user, `"stale"`); rec.Code != http.StatusPreconditionFailed {
				t.Fatalf("DELETE with a stale ETag = %d %s, want 412", rec.Code, rec.Body)
			}
			if rec := deleteRelation(t, app, user, etag); rec.Code != http.StatusNoContent {
				t.Fatalf("DELETE with the GET's ETag = %d %s, want 204", rec.Code, rec.Body)
			}
			if rec := deleteRelation(t, app, user, etag); rec.Code != http.StatusNotFound {
				t.Fatalf("second DELETE = %d %s, want 404", rec.Code, rec.Body)
			}
		})
	}
}

// A caller who cannot read the row gets the 404 of a missing row, never a
// 412: a stale If-Match must not tell an outsider that the row exists.
func TestDelete_IfMatchHiddenRowIs404(t *testing.T) {
	app := deleteApp(t)
	if rec := deleteEntity(t, app, "carol", `"stale"`); rec.Code != http.StatusNotFound {
		t.Fatalf("entity DELETE by an outsider = %d %s, want 404", rec.Code, rec.Body)
	}
	if rec := deleteRelation(t, app, "carol", `"stale"`); rec.Code != http.StatusNotFound {
		t.Fatalf("relation DELETE by an outsider = %d %s, want 404", rec.Code, rec.Body)
	}
	if getEntityETag(t, app, "bob") == "" {
		t.Fatal("the row is gone")
	}
}
