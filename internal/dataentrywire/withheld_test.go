package dataentrywire_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/dataentrywire"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// withheldProbe reads rela.secrets.token and reports what it saw, so a test
// can tell the operator's secret from the withheld error.
const withheldProbe = `local _, v = pcall(function() return rela.secrets.token end)
`

// writeWithheldDataEntryProject writes a project whose .rela directory holds
// a secret, with one action and one document that each read it.
func writeWithheldDataEntryProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "schema.yaml", `version: "1.0"
entities:
  doc:
    label: Document
    id_prefix: DOC
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)
	writeFile(t, root, "data-entry.yaml", `app:
  name: Test
actions:
  probe:
    label: Probe
    script: probe.lua
    capabilities: {secrets: [token]}
documents:
  probe:
    title: Probe
    entity_type: doc
    script: probe.lua
    capabilities: {secrets: [token]}
`)
	writeFile(t, root, "actions/probe.lua", withheldProbe+`return {message = "saw: " .. tostring(v)}
`)
	writeFile(t, root, "scripts/probe.lua", withheldProbe+`print("saw: " .. tostring(v))
`)
	writeFile(t, root, "entities/docs/DOC-1.md", "---\nid: DOC-1\ntype: doc\ntitle: First\n---\n")
	writeFile(t, root, ".rela/secrets.yaml", "token: operator-token\n")
	return root
}

// newWiredAppWith is newWiredApp with extra build options.
func newWiredAppWith(t *testing.T, root string, opts ...appbuild.Option) *dataentry.App {
	t.Helper()
	fsys := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.At(root, fsys)
	require.NoError(t, err)
	sink, err := audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))
	require.NoError(t, err)
	svc, err := appbuild.New(appbuild.Config{
		FS: fsys, Paths: paths, ScriptEngine: script.NewEngine(), Audit: sink,
	}, append([]appbuild.Option{appbuild.WithACL(acl.NopACL{})}, opts...)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	resolver, err := dataentry.ResolverFromServices(svc, appbuild.FlowEdits(svc))
	require.NoError(t, err)
	app, err := dataentry.NewApp(
		fsys, paths, svc.ProjectFiles(), svc.Templater(), svc.Meta(), svc.Store(), svc.Versions(),
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), svc.ACL(),
		resolver, svc.Audit(), svc.State(), dataentry.UngatedCommandAuthorizer(), appbuild.CompiledWorlds(svc),
	)
	require.NoError(t, err)
	require.NoError(t, dataentrywire.Services(app, svc))
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: principal.SystemUser(), Tool: principal.ToolDesktop}
	})
	return app
}

// probeBodies runs the action and renders the document, and returns both
// response bodies.
func probeBodies(t *testing.T, app *dataentry.App) (action, document string) {
	t.Helper()
	router := app.NewRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/probe", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	action = rec.Body.String()

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_documents/probe/DOC-1", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return action, rec.Body.String()
}

// Action scripts and document renders run on the App's own Lua deps. Under a
// withheld host config they must see the withheld error, not the secret in
// the shared .rela directory (TKT-3RWQ1PK). Without dataentry.SetLuaHost the
// runtime falls back to that directory and reads the operator's token.
func TestServices_RequestScriptsSeeWithheldHostConfig(t *testing.T) {
	const reason = "withheld by the test host"

	t.Run("withheld", func(t *testing.T) {
		app := newWiredAppWith(t, writeWithheldDataEntryProject(t), appbuild.WithWithheldHostConfig(reason))
		action, document := probeBodies(t, app)
		for name, body := range map[string]string{"action": action, "document": document} {
			require.Contains(t, body, reason, name)
			require.NotContains(t, body, "operator-token", name)
		}
	})

	t.Run("shared", func(t *testing.T) {
		app := newWiredAppWith(t, writeWithheldDataEntryProject(t))
		action, document := probeBodies(t, app)
		require.Contains(t, action, "saw: operator-token")
		require.Contains(t, document, "saw: operator-token")
	})
}
