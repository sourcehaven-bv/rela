//go:build sqlite

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// useTestDocumentsRoot keeps document workspaces in a temporary directory.
func useTestDocumentsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	prev := documentsRoot
	documentsRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { documentsRoot = prev })
	return root
}

func TestIsRelaDocument(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Budget.rela")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	bundle := filepath.Join(dir, "Old.rela")
	require.NoError(t, os.Mkdir(bundle, 0o755))
	other := filepath.Join(dir, "notes.txt")
	require.NoError(t, os.WriteFile(other, nil, 0o600))

	assert.True(t, isRelaDocument(file))
	upper := filepath.Join(dir, "Upper.RELA")
	require.NoError(t, os.WriteFile(upper, nil, 0o600))
	assert.True(t, isRelaDocument(upper), "the extension is matched without regard to case")
	assert.False(t, isRelaDocument(bundle), "a .rela folder is a project folder, not a document")
	assert.False(t, isRelaDocument(other))
	assert.False(t, isRelaDocument(filepath.Join(dir, "missing.rela")))
	assert.Equal(t, "Budget", documentName(file))
}

func TestDocumentContext(t *testing.T) {
	wsRoot := useTestDocumentsRoot(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "A.rela")
	b := filepath.Join(dir, "B.rela")

	pa, err := documentContext(a)
	require.NoError(t, err)
	assert.Equal(t, a, pa.DatabaseFile)
	assert.Equal(t, a, appbuild.DatabasePath(pa), "the document is the database")
	assert.Equal(t, wsRoot, filepath.Dir(pa.Root), "the workspace is private, not next to the file")
	assert.DirExists(t, pa.CacheDir)

	again, err := documentContext(a)
	require.NoError(t, err)
	assert.Equal(t, pa.Root, again.Root, "reopening finds the same workspace")

	pb, err := documentContext(b)
	require.NoError(t, err)
	assert.NotEqual(t, pa.Root, pb.Root, "each document has its own workspace")
}

// A document made from a template carries its config and data, opens
// without the template folder, and is not shadowed by a schema.yaml that
// happens to sit beside it.
func TestNewDocumentFromTemplate(t *testing.T) {
	useTestDocumentsRoot(t)
	tmpl := t.TempDir()
	writeTestFile(t, tmpl, "schema.yaml", testSchema)
	writeTestFile(t, tmpl, "data-entry.yaml", testDataEntry)
	writeTestFile(t, tmpl, "entities/docs/DOC-1.md", "---\nid: DOC-1\ntype: doc\ntitle: First\n---\n")

	dir := t.TempDir()
	writeTestFile(t, dir, "schema.yaml", "not: [a schema")
	dest := filepath.Join(dir, "Budget.rela")
	require.NoError(t, newDocumentFromTemplate(context.Background(), tmpl, dest))
	require.True(t, isRelaDocument(dest))
	require.NoError(t, os.RemoveAll(tmpl))

	d := newTestDesktop(t)
	require.Empty(t, d.loadProject(dest, false))
	assert.Equal(t, dest, d.activePath)
	assert.Equal(t, dest, d.prefs.RecentProjects[0].Path, "Open Recent reopens the file")
	assert.Equal(t, "Budget", d.prefs.RecentProjects[0].Name)
	got, err := d.svc.Store().GetEntity(context.Background(), entity.Ref{ID: "DOC-1"})
	require.NoError(t, err)
	assert.Equal(t, "First", got.Properties["title"])

	// The database actions work on the document, not its workspace.
	out := t.TempDir()
	_, err = d.withProjectReleased(exportConfig(out))
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(out, "schema.yaml"))
	assert.Equal(t, dest, d.activePath)

	err = newDocumentFromTemplate(context.Background(), out, dest)
	require.ErrorContains(t, err, "already exists")
}

func TestNewDocumentFromTemplate_RefusesAndCleansUp(t *testing.T) {
	useTestDocumentsRoot(t)
	dir := t.TempDir()

	noSchema := t.TempDir()
	writeTestFile(t, noSchema, "data-entry.yaml", testDataEntry)
	dest := filepath.Join(dir, "A.rela")
	require.ErrorContains(t, newDocumentFromTemplate(context.Background(), noSchema, dest), "no schema.yaml")
	assert.NoFileExists(t, dest)

	badData := t.TempDir()
	writeTestFile(t, badData, "schema.yaml", testSchema)
	writeTestFile(t, badData, "entities/docs/DOC-1.md", "---\nid: [DOC-1\n---\n")
	dest = filepath.Join(dir, "B.rela")
	require.Error(t, newDocumentFromTemplate(context.Background(), badData, dest))
	assert.NoFileExists(t, dest, "a failed creation leaves no half-made document")
}

// A document with no data-entry.yaml is set up inside the document, so the
// generated config travels with it.
func TestDocument_SetupStoresConfigInside(t *testing.T) {
	useTestDocumentsRoot(t)
	tmpl := t.TempDir()
	writeTestFile(t, tmpl, "schema.yaml", testSchema)
	dir := t.TempDir()
	dest := filepath.Join(dir, "Notes.rela")
	require.NoError(t, newDocumentFromTemplate(context.Background(), tmpl, dest))

	d := newTestDesktop(t)
	require.Equal(t, "needs_setup", d.loadProject(dest, false))
	require.Empty(t, d.GenerateDataEntryConfig("Notes"))
	require.NotNil(t, d.app)
	assert.NoFileExists(t, filepath.Join(dir, "data-entry.yaml"))
}
