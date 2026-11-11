//go:build sqlite || postgres

package appbuild_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const (
	raceHost       = "127.0.0.1:8080"
	raceUserHeader = "X-Race-User"
)

// raceRouter is the data-entry router over svc, wired as rela-server's
// buildApp does, with a header login.
func raceRouter(t *testing.T, svc *appbuild.Services) http.Handler {
	t.Helper()
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
	return app.NewRouter()
}

// TestOutsideEdits_RunStartedDuringHTTPPatchFailsClosed pins the race closed
// for the data-entry PATCH on the database tiers (TKT-XI7IDBS). The handler
// checks the fields, finds no run and waits for the store's write lock. A
// run start commits under that lock first. The write then checks the flow
// ceiling again inside its transaction and is refused with 403.
func TestOutsideEdits_RunStartedDuringHTTPPatchFailsClosed(t *testing.T) {
	root := writeFlowProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "data-entry.yaml"), []byte("app:\n  name: Race\n"), 0o600))
	svc, _ := outsideEditsProjectIn(t, root)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	router := raceRouter(t, svc)
	def, ok := svc.Meta().GetEntityDef("person")
	require.True(t, ok)

	finish, held := holdWriteLock(t, svc, func(view store.Store) error {
		runs, err := appbuild.JoinedFlowRuns(svc, view)
		if err != nil {
			return err
		}
		_, err = runs.Create(context.Background(), raceRun())
		return err
	})
	rec := httptest.NewRecorder()
	patched := make(chan error, 1)
	go func() {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch,
			"/api/v1/"+def.GetPlural("person")+"/PER-1", strings.NewReader(`{"properties":{"documents":true}}`))
		req.Host = raceHost
		req.Header.Set("Origin", "http://"+raceHost)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(raceUserHeader, "dave")
		router.ServeHTTP(rec, req)
		patched <- nil
	}()
	waitForWriteLockWaiter(t, svc, patched, "entitymanager.UpdateGrant.PatchEntity")
	finish()
	require.NoError(t, <-held)
	<-patched
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"field-affordance:read-only:documents"`)
}
