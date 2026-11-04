//go:build sqlite

package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// newSQLiteBatchApp serves the batch endpoint over a real sqlitestore, so
// the capability comes from the store and a failure really rolls back.
func newSQLiteBatchApp(t *testing.T) *App {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "batch.db"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(context.Background(), db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	meta := noteMeta(t)
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	require.NoError(t, fs.MkdirAll(paths.CacheDir, 0o755))
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	app.broker = newEventBroker()
	return app
}

func TestBatchSQLite_CapabilityIsAdvertised(t *testing.T) {
	app := newSQLiteBatchApp(t)
	require.True(t, app.batch.atomic)

	rec := httptest.NewRecorder()
	app.handleV1Schema(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
	var got v1.Schema
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.True(t, got.Capabilities.AtomicBatch)
}

// TestBatchSQLite_ConflictRollsBack: the second create collides, so the
// first one, already written inside the transaction, is gone too.
func TestBatchSQLite_ConflictRollsBack(t *testing.T) {
	app := newSQLiteBatchApp(t)
	seedEntity(app, &entity.Entity{ID: "N-1", Type: "note", Properties: map[string]any{"title": "old"}})

	rec := postBatch(context.Background(), t, app, `{"entities":[
		{"type":"note","id":"N-2","properties":{"title":"a"}},
		{"ref":"dup","type":"note","id":"N-1","properties":{"title":"b"}}
	]}`)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	e := decodeBatchError(t, rec)
	require.Equal(t, &v1.ErrorOperation{Kind: "entity", Index: 1, Ref: "dup"}, e.Operation)
	require.Equal(t, 1, countOf(t, app.store, "note"))
}

func TestBatchSQLite_Creates(t *testing.T) {
	app := newSQLiteBatchApp(t)
	rec := postBatch(context.Background(), t, app, `{
		"entities":[
			{"ref":"t","type":"ticket","properties":{"title":"a"}},
			{"ref":"f","type":"feature","properties":{"title":"b"}}
		],
		"relations":[{"from":{"ref":"t"},"type":"implements","to":{"ref":"f"}}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, countOf(t, app.store, "ticket"))
	require.Equal(t, 1, countRelations(t, app.store))
}
