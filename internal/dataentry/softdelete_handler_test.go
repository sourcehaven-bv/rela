package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func softDeleteUser(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
}

// serveAs sends one request through the v1 router as ctx's principal.
func serveAs(ctx context.Context, app *App, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1DynamicRoutes(rec, req)
	return rec
}

// TestSoftDelete_DeleteThenRestore pins the Undo round trip: a web DELETE
// hides the entity and its relations, and POST .../restore brings both back.
func TestSoftDelete_DeleteThenRestore(t *testing.T) {
	app := newTestAppV1(t)
	require.NotNil(t, app.write.softDeletes, "the test store supports soft delete")
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
	alice := softDeleteUser("alice")

	rec := serveAs(alice, app, http.MethodDelete, "/api/v1/tickets/TKT-001")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusNotFound, serveAs(alice, app, http.MethodGet, "/api/v1/tickets/TKT-001").Code)
	_, err := app.store.GetRelation(context.Background(), "TKT-001", "implements", "FEAT-001")
	require.ErrorIs(t, err, store.ErrNotFound)

	rec = serveAs(alice, app, http.MethodPost, "/api/v1/tickets/TKT-001/restore")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusOK, serveAs(alice, app, http.MethodGet, "/api/v1/tickets/TKT-001").Code)
	_, err = app.store.GetRelation(context.Background(), "TKT-001", "implements", "FEAT-001")
	require.NoError(t, err)

	// A second undo finds nothing to restore.
	rec = serveAs(alice, app, http.MethodPost, "/api/v1/tickets/TKT-001/restore")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSoftDelete_RestoreRefusals(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "T2"}})
	alice := softDeleteUser("alice")
	require.Equal(t, http.StatusNoContent, serveAs(alice, app, http.MethodDelete, "/api/v1/tickets/TKT-001").Code)

	for _, tc := range []struct {
		name, method, path string
		want               int
	}{
		{"never deleted", http.MethodPost, "/api/v1/tickets/TKT-002/restore", http.StatusNotFound},
		{"no such id", http.MethodPost, "/api/v1/tickets/TKT-999/restore", http.StatusNotFound},
		{"wrong type", http.MethodPost, "/api/v1/features/TKT-001/restore", http.StatusNotFound},
		{"faced address", http.MethodPost, "/api/v1/tickets/TKT-001@draft/restore", http.StatusNotFound},
		{"not a POST", http.MethodGet, "/api/v1/tickets/TKT-001/restore", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, serveAs(alice, app, tc.method, tc.path).Code)
		})
	}
}

// TestSoftDelete_RestoreReadGate pins who may learn that a soft-deleted
// entity exists: a reader, or the principal who deleted it. Anyone else gets
// the same 404 body as for an id that was never deleted.
func TestSoftDelete_RestoreReadGate(t *testing.T) {
	app := newTestAppV1(t)
	for _, id := range []string{"TKT-001", "TKT-002"} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"none":   {},
			"reader": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"bob": "none", "carol": "reader", "alice": "none"},
	}, app.store)
	app.acl = d
	restoreAs := func(user, id string) *httptest.ResponseRecorder {
		ctx := gateCtxFor(softDeleteUser(user), t, d)
		return serveAs(ctx, app, http.MethodPost, "/api/v1/tickets/"+id+"/restore")
	}

	// The manager in this fixture runs without the ACL, so the delete
	// itself is not what is under test here.
	for _, id := range []string{"TKT-001", "TKT-002"} {
		_, err := app.write.softDeletes.SoftDeleteEntity(softDeleteUser("alice"), id)
		require.NoError(t, err)
	}

	hidden := restoreAs("bob", "TKT-001")
	missing := restoreAs("bob", "TKT-999")
	assert.Equal(t, http.StatusNotFound, hidden.Code)
	assert.Equal(t, strings.ReplaceAll(missing.Body.String(), "TKT-999", "TKT-001"), hidden.Body.String(),
		"a hidden mark must look like no mark")

	assert.Equal(t, http.StatusNoContent, restoreAs("carol", "TKT-001").Code, "a reader may restore")
	assert.Equal(t, http.StatusNoContent, restoreAs("alice", "TKT-002").Code,
		"the deleter may restore without the read grant")
}
