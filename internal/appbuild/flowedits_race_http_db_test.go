//go:build sqlite || postgres

package appbuild_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const (
	raceHost       = "127.0.0.1:8080"
	raceUserHeader = "X-Race-User"
	// grantPatchFrame is the frame of a data-entry write through an update
	// grant: PATCH and history restore.
	grantPatchFrame = "entitymanager.UpdateGrant.PatchEntity"
)

// raceDataEntry is the outside-edits project with a file property `scan`
// that onboarding's collect state denies, and the data-entry router over
// it, wired as rela-server's buildApp does, with a header login.
func raceDataEntry(t *testing.T) (*appbuild.Services, http.Handler) {
	t.Helper()
	root := writeFlowProject(t)
	schema, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", "schema.yaml"))
	require.NoError(t, err)
	const documents = "      documents:\n        type: boolean\n"
	require.Contains(t, string(schema), documents)
	writeMetamodelBody(t, root, strings.Replace(string(schema), documents,
		documents+"      scan:\n        type: file\n", 1))
	require.NoError(t, os.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte("app:\n  name: Race\n"), 0o600))
	svc, _ := outsideEditsProjectIn(t, root, "scan")

	resolver, err := dataentry.ResolverFromServices(svc, appbuild.FlowEdits(svc))
	require.NoError(t, err)
	app, err := dataentry.NewApp(
		svc.FS(), svc.Paths(), svc.ProjectFiles(), svc.Templater(), svc.Meta(), svc.Store(), svc.Versions(),
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		resolver, svc.Audit(), svc.State(), dataentry.UngatedCommandAuthorizer(), appbuild.CompiledWorlds(svc),
	)
	require.NoError(t, err)
	require.NoError(t, dataentrywire.Services(app, svc))
	require.NoError(t, app.SetSecurityConfig(dataentry.SecurityConfig{BindAddress: raceHost}))
	app.SetPrincipalResolver(dataentry.HeaderPrincipalResolver(raceUserHeader))
	app.SetPrincipalHeader(raceUserHeader)
	return svc, app.NewRouter()
}

// raceRequest is a same-origin request by dave, a clerk who may update a
// person.
func raceRequest(t *testing.T, method, path, contentType string, body io.Reader) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, body)
	req.Host = raceHost
	req.Header.Set("Origin", "http://"+raceHost)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set(raceUserHeader, "dave")
	return req
}

// raceHTTP serves req while a run of onboarding starts on PER-1 under the
// store's write lock. The handler checks the fields, finds no run and waits
// for the lock in the goroutine named by frame. The run commits first.
func raceHTTP(t *testing.T, svc *appbuild.Services, router http.Handler, req *http.Request, frame string,
) *httptest.ResponseRecorder {
	t.Helper()
	finish, held := holdWriteLock(t, svc, func(view store.Store) error {
		runs, err := appbuild.JoinedFlowRuns(svc, view)
		if err != nil {
			return err
		}
		_, err = runs.Create(context.Background(), raceRun())
		return err
	})
	rec := httptest.NewRecorder()
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("response: %d %s", rec.Code, rec.Body)
		}
	})
	served := make(chan error, 1)
	go func() {
		router.ServeHTTP(rec, req)
		served <- nil
	}()
	waitForWriteLockWaiter(t, svc, served, frame)
	finish()
	require.NoError(t, <-held)
	<-served
	return rec
}

// requireRaceRefused asserts the 403 the handler's own check gives for a
// property the run denies.
func requireRaceRefused(t *testing.T, rec *httptest.ResponseRecorder, prop string) {
	t.Helper()
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"field-affordance:read-only:`+prop+`"`)
}

// waitForHistory waits until the version sweep has recorded n versions of
// PER-1.
func waitForHistory(t *testing.T, router http.Handler, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, raceRequest(t, http.MethodGet, "/api/v1/_history/person/PER-1", "", http.NoBody))
		var body struct {
			Versions []json.RawMessage `json:"versions"`
		}
		return rec.Code == http.StatusOK && json.Unmarshal(rec.Body.Bytes(), &body) == nil && len(body.Versions) >= n
	}, writeLockWait, 20*time.Millisecond)
}

func personsPath(t *testing.T, svc *appbuild.Services) string {
	t.Helper()
	def, ok := svc.Meta().GetEntityDef("person")
	require.True(t, ok)
	return "/api/v1/" + def.GetPlural("person")
}

// The tests below pin the race closed for the data-entry writes on the
// database tiers (TKT-XI7IDBS): each write checks the flow ceiling again
// inside its transaction, sees the run that committed while it waited, and
// is refused with 403.

func TestOutsideEdits_RunStartedDuringHTTPPatchFailsClosed(t *testing.T) {
	svc, router := raceDataEntry(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	req := raceRequest(t, http.MethodPatch, personsPath(t, svc)+"/PER-1", "application/json",
		strings.NewReader(`{"properties":{"documents":true}}`))
	requireRaceRefused(t, raceHTTP(t, svc, router, req, grantPatchFrame), "documents")
}

func TestOutsideEdits_RunStartedDuringHistoryRestoreFailsClosed(t *testing.T) {
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "20ms")
	t.Setenv("RELA_VERSION_SWEEP_IDLE", "1ms")
	svc, router := raceDataEntry(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada", "documents": false})
	waitForHistory(t, router, 1)
	_, err := svc.EntityManager().PatchEntity(flowUser(t, svc, "dave"), "PER-1",
		entity.Patch{Properties: map[string]any{"documents": true}})
	require.NoError(t, err)
	waitForHistory(t, router, 2)

	// Version 1 has documents false: restoring it writes documents.
	req := raceRequest(t, http.MethodPost, "/api/v1/_history/person/PER-1/1/restore", "", http.NoBody)
	requireRaceRefused(t, raceHTTP(t, svc, router, req, grantPatchFrame), "documents")
}
