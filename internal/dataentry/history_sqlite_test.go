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

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// TKT-4NU9ZD AC-7: history must reach the data-entry UI on the sqlite build.
//
// Every other test on this handler drives it through a stub VersionService, so
// they prove the handler works given a capability — never that THIS backend
// supplies one. That gap is not academic: sqlitestore implemented the whole
// store.VersionService and its conformance suite passed while appbuild still
// handed the App a nil, so the handler returned 501 for the entire feature and
// no test anywhere disagreed.
//
// So this file uses a real sqlitestore and asserts on the HTTP response.

// sqliteVersions opens a real store and returns its version service, the way
// appbuild's versionServiceFor does.
func sqliteVersions(t *testing.T) (store.Store, store.VersionService) {
	t.Helper()

	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "history.db"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	st, err := sqlitestore.New(db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	provider, ok := any(st).(store.VersionServiceProvider)
	require.True(t, ok, "sqlitestore must provide store.VersionService")
	svc := provider.VersionStore()
	require.NotNil(t, svc)
	return st, svc
}

// TestHistoryIsServedOnSQLite is AC-7: the 501 history_unsupported path must
// no longer be taken on this build.
func TestHistoryIsServedOnSQLite(t *testing.T) {
	st, svc := sqliteVersions(t)
	ctx := context.Background()

	// A captured version for an entity the fixture also knows about, so the
	// handler's row gate resolves a live entity rather than 404ing first.
	tkt := entity.New("TKT-1", "ticket")
	tkt.Properties = map[string]any{"title": "a ticket"}
	require.NoError(t, st.CreateEntity(ctx, tkt))
	require.NoError(t, svc.WriteVersion(ctx, store.VersionInput{
		EntityID:   "TKT-1",
		Type:       "ticket",
		Op:         store.VersionOpUpdate,
		Content:    "the body as it was",
		Properties: map[string]any{"title": "a ticket"},
		SchemaHash: "schema-1",
		Projection: []byte(`{"v":1}`),
	}))

	f := newFixture()
	f.AddNode(tkt)
	app := newAppFromParts(nil, testMeta(), f)
	app.versions = svc

	rec := doHistoryGet(app, "ticket", "TKT-1")
	require.NotEqual(t, http.StatusNotImplemented, rec.Code,
		"history_unsupported on the sqlite build: the capability is implemented "+
			"but not wired (see appbuild versionServiceFor)")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	// The timeline lists metadata only; one entry means the capture was found.
	var timeline struct {
		Versions []struct {
			Version int    `json:"version"`
			Op      string `json:"op"`
		} `json:"versions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &timeline))
	require.Len(t, timeline.Versions, 1, "the captured version is not in the timeline")

	// And the snapshot route serves the content the version actually holds,
	// which is what proves the read path reaches sqlitestore's tables rather
	// than merely returning an empty-but-successful timeline.
	snap := doHistoryGet(app, "ticket", "TKT-1/1")
	require.Equal(t, http.StatusOK, snap.Code, "body=%s", snap.Body.String())
	require.Contains(t, snap.Body.String(), "the body as it was")
}

// TestConfigHistoryEnabledOnSQLite pins the SPA-facing flag for this build.
//
// The flag exists so the History button is not rendered on a backend certain
// to 501. With the capability unwired it read false, so the button was hidden
// and the feature was invisible rather than broken — which is why nobody
// noticed it was missing.
func TestConfigHistoryEnabledOnSQLite(t *testing.T) {
	_, svc := sqliteVersions(t)

	app := newAppFromParts(nil, testMeta(), &fixture{})
	app.versions = svc

	rec := httptest.NewRecorder()
	app.handleV1Config(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code)

	var cfg struct {
		App struct {
			HistoryEnabled bool `json:"history_enabled"`
		} `json:"app"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cfg))
	require.True(t, cfg.App.HistoryEnabled,
		"history_enabled is false on the sqlite build, so the SPA hides the History button")
}
