package dataentry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// newFileStoreTestApp is newTestAppV1 over a file store on an in-memory
// file system.
func newFileStoreTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)
	fss, err := fsstore.New(fsstore.Config{
		FS: fs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations", AttachmentsKey: "attachments", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{"ticket": {Plural: "tickets"}, "feature": {Plural: "features"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = fss.Close() })
	paths := &project.Context{Root: "/"}
	rebindApp(app, fs, paths, appbuildtest.New(app.Meta(), appbuildtest.WithStore(fss), appbuildtest.WithFS(fs, paths)))
	return app
}

// TestWrite_ConflictBlockBodyIs422 pins BUG-FJ1ITQT: a body the file store
// refuses because it holds a complete git conflict block answers 422
// conflict_block on every HTTP write path, with a detail that does not echo
// the body.
func TestWrite_ConflictBlockBodyIs422(t *testing.T) {
	const secret = "SECRET-BODY-LINE"
	block := "```text\n<<<<<<< HEAD\n" + secret + "\n=======\ntheirs\n>>>>>>> feature\n```\n"
	content, err := json.Marshal(block)
	require.NoError(t, err)

	tests := []struct {
		name string
		send func(app *App, rec *httptest.ResponseRecorder)
	}{
		{"create entity", func(app *App, rec *httptest.ResponseRecorder) {
			app.write.handleV1CreateEntity(rec, httptest.NewRequest(http.MethodPost, "/api/v1/tickets",
				strings.NewReader(`{"properties":{"title":"x"},"content":`+string(content)+`}`)), "ticket", "tickets")
		}},
		{"patch entity", func(app *App, rec *httptest.ResponseRecorder) {
			app.write.handleV1UpdateEntity(rec, httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
				strings.NewReader(`{"content":`+string(content)+`}`)), "ticket", "tickets", "TKT-001")
		}},
		{"create entity with relation content", func(app *App, rec *httptest.ResponseRecorder) {
			app.write.handleV1CreateEntity(rec, httptest.NewRequest(http.MethodPost, "/api/v1/tickets",
				strings.NewReader(`{"properties":{"title":"x"},"relations":{"implements":{"data":[`+
					`{"type":"feature","id":"FEAT-001","content":`+string(content)+`}]}}}`)), "ticket", "tickets")
		}},
		{"patch relation content", func(app *App, rec *httptest.ResponseRecorder) {
			app.write.handleV1UpdateEntity(rec, httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
				strings.NewReader(`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001","content":`+
					string(content)+`}]}}}`)), "ticket", "tickets", "TKT-001")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newFileStoreTestApp(t)
			implements := app.State().Meta.Relations["implements"]
			implements.Content = true
			app.State().Meta.Relations["implements"] = implements
			seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}})
			seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "f"}})

			rec := httptest.NewRecorder()
			tc.send(app, rec)
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "conflict_block")
			require.NotContains(t, rec.Body.String(), secret, "the problem must not echo the body")
		})
	}
}

// TestWriteInternalError_ConflictBlockIs422 pins the backstop: a path that
// would answer a conflict-block refusal as a server error answers the 422.
func TestWriteInternalError_ConflictBlockIs422(t *testing.T) {
	rec := httptest.NewRecorder()
	writeInternalError(rec, httptest.NewRequest(http.MethodPost, "/api/v1/tickets", http.NoBody),
		"write_failed", "Write failed", fmt.Errorf("write entity: %w", store.ErrConflictBlock))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "conflict_block")
}
