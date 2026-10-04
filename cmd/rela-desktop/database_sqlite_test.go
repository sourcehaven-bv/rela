//go:build sqlite

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/desktop"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const testSchema = `version: "1.0"
entities:
  doc:
    label: Document
    id_prefix: DOC
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`

const testDataEntry = `app:
  name: Test
`

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// newTestDesktop returns a Desktop whose preferences are written under a
// temporary home, so loading a project does not touch the user's own.
func newTestDesktop(t *testing.T) *Desktop {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	d := &Desktop{prefs: &desktop.Preferences{}, registry: newProjectRegistry()}
	t.Cleanup(d.releaseLoadedProject)
	return d
}

// A project whose config lives only in its database is recognized, opens
// without the setup prompt, and round-trips its config and data through the
// import and export actions.
func TestDatabaseProject_OpensAndRoundTrips(t *testing.T) {
	src := t.TempDir()
	writeTestFile(t, src, "schema.yaml", testSchema)
	writeTestFile(t, src, "data-entry.yaml", testDataEntry)
	writeTestFile(t, src, "entities/docs/DOC-1.md", "---\nid: DOC-1\ntype: doc\ntitle: First\n---\n")

	root := t.TempDir()
	require.False(t, isRelaProject(root), "an empty directory is not a project")
	fsys := storage.NewSafeFS(storage.NewOsFS())
	require.NoError(t, os.Mkdir(filepath.Join(root, project.CacheDir), 0o755))
	paths, err := project.Discover(root, fsys)
	require.NoError(t, err)
	_, err = appbuild.LoadProjectConfig(context.Background(), fsys, paths, src)
	require.NoError(t, err)
	require.True(t, isRelaProject(root), "a directory holding only rela.db is a project")

	d := newTestDesktop(t)
	require.Empty(t, d.loadProject(root, false), "data-entry.yaml comes from the database")
	require.False(t, d.NeedsSetup())

	summary, err := d.withProjectReleased(importData(src))
	require.NoError(t, err)
	assert.Contains(t, summary, "1 entities")
	require.NotNil(t, d.app, "the project is open again after the import")

	out := t.TempDir()
	_, err = d.withProjectReleased(exportConfig(out))
	require.NoError(t, err)
	_, err = d.withProjectReleased(exportData(out))
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(out, "schema.yaml"))
	assert.FileExists(t, filepath.Join(out, "data-entry.yaml"))
	assert.FileExists(t, filepath.Join(out, "entities", "docs", "DOC-1.md"))

	_, err = d.withProjectReleased(exportConfig(out))
	require.Error(t, err, "exporting over existing files is refused")
	require.NotNil(t, d.app, "a failed export still reopens the project")

	_, err = d.withProjectReleased(importConfig(out))
	require.NoError(t, err)
}

// A project whose schema lives only in its database, with no
// data-entry.yaml, can still be set up: setup reads the schema the services
// loaded, not the disk.
func TestDatabaseProject_SetupFromDatabaseSchema(t *testing.T) {
	src := t.TempDir()
	writeTestFile(t, src, "schema.yaml", testSchema)
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, project.CacheDir), 0o755))
	fsys := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fsys)
	require.NoError(t, err)
	_, err = appbuild.LoadProjectConfig(context.Background(), fsys, paths, src)
	require.NoError(t, err)

	d := newTestDesktop(t)
	require.Equal(t, "needs_setup", d.loadProject(root, false))
	info := d.GetSetupInfo()
	require.NotContains(t, info, "error")
	assert.Equal(t, []string{"doc"}, info["entity_types"])
	require.Empty(t, d.GenerateDataEntryConfig("Test"))
	require.NotNil(t, d.app)
}

// A load that fails after the services opened releases the database, so
// fixing the cause and opening again works.
func TestDatabaseProject_FailedLoadReleasesTheDatabase(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "schema.yaml", testSchema)
	writeTestFile(t, root, "data-entry.yaml", "app: [\n")
	d := newTestDesktop(t)
	require.NotEmpty(t, d.loadProject(root, false))

	writeTestFile(t, root, "data-entry.yaml", testDataEntry)
	require.Empty(t, d.loadProject(root, false), "the failed load must not keep the database locked")
}

// The desktop wires the same services onto the app as the server: a list's
// condition: narrows its rows instead of failing as "not compiled".
func TestDesktop_ListConditionApplies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "schema.yaml", testSchema)
	writeTestFile(t, root, "data-entry.yaml", testDataEntry+`lists:
  first_only:
    entity_type: doc
    condition: "entity.title == 'First'"
`)
	writeTestFile(t, root, "entities/docs/DOC-1.md", "---\nid: DOC-1\ntype: doc\ntitle: First\n---\n")
	writeTestFile(t, root, "entities/docs/DOC-2.md", "---\nid: DOC-2\ntype: doc\ntitle: Second\n---\n")
	d := newTestDesktop(t)
	require.Empty(t, d.loadProject(root, false))

	rec := httptest.NewRecorder()
	d.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/docs?list_id=first_only", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Data []struct{ ID string } `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Data, 1, rec.Body.String())
	assert.Equal(t, "DOC-1", resp.Data[0].ID)
}

// An acl.yaml does not restrict the desktop user: a project with one that
// grants nothing still opens and serves its API, even with no OS user name
// to attribute requests to, which the ACL would refuse.
func TestDesktop_IgnoresACL(t *testing.T) {
	t.Setenv("USER", "")
	t.Setenv("RELA_DATAENTRY_USER", "")
	root := t.TempDir()
	writeTestFile(t, root, "schema.yaml", testSchema)
	writeTestFile(t, root, "data-entry.yaml", testDataEntry)
	writeTestFile(t, root, "acl.yaml", "roles:\n  nobody: {}\n")
	d := newTestDesktop(t)
	require.Empty(t, d.loadProject(root, false))

	rec := httptest.NewRecorder()
	d.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// A project with a schema but no data-entry.yaml anywhere still asks for
// setup.
func TestDatabaseProject_NeedsSetupWithoutDataEntry(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "schema.yaml", testSchema)
	d := newTestDesktop(t)
	require.Equal(t, "needs_setup", d.loadProject(root, false))
	require.True(t, d.NeedsSetup())
	require.Nil(t, d.app)
}

func TestWithProjectReleased_NoProject(t *testing.T) {
	d := newTestDesktop(t)
	_, err := d.withProjectReleased(exportConfig(t.TempDir()))
	require.Error(t, err)
}
