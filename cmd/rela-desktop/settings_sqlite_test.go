//go:build sqlite

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
)

// Secrets go to the keychain and the AI and mail settings into the project's
// database, and both survive reopening the project.
func TestProjectSettings_OpenProject(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "schema.yaml", testSchema)
	writeTestFile(t, root, "data-entry.yaml", testDataEntry)

	d := newTestDesktop(t)
	kc := newFakeKeychain()
	store, err := newKeychainSecrets(kc)
	require.NoError(t, err)
	d.keychain = store
	require.Empty(t, d.loadProject(root, false))
	approve := true
	s, err := newProjectSettings(d, func(_, _, _ string) bool { return approve })
	require.NoError(t, err)
	s.windows.add("settings-1", projectID(d.app.ProjectRoot()))
	ctx := fromWindow("settings-1")

	require.Empty(t, s.SetSecret(ctx, "ai_api_key", "sk-test"))
	assert.NotEmpty(t, s.SetSecret(ctx, "bad name", "x"))
	view := s.Load(ctx)
	require.Empty(t, view.Error)
	assert.Equal(t, []string{"ai_api_key"}, view.Secrets)
	for item := range kc.items {
		assert.NotContains(t, item, root, "keychain items are keyed by document id, not path")
	}

	ai := "base_url: http://localhost:1/v1\nmodel: m\n"
	require.Empty(t, s.SaveFile(ctx, hostconfig.AIFile, ai))
	assert.NotEmpty(t, s.SaveFile(ctx, hostconfig.AIFile, "model: m\n"), "an invalid ai.yaml is refused")
	assert.NotEmpty(t, s.SaveFile(ctx, "secrets.yaml", "x: y\n"))
	require.Empty(t, s.SaveFile(ctx, hostconfig.MailFile, "transport: memory\nfrom: rela@example.com\n"))

	_, err = os.Stat(filepath.Join(root, ".rela", hostconfig.AIFile))
	assert.True(t, os.IsNotExist(err), "settings are not written to .rela")

	require.Empty(t, d.loadProject(root, false))
	view = s.Load(ctx)
	assert.Equal(t, ai, view.AI)
	assert.Contains(t, view.Mail, "transport: memory")
	assert.Equal(t, []string{"ai_api_key"}, view.Secrets)

	data, err := d.svc.State().Get(context.Background(), hostFileKeyPrefix+hostconfig.AIFile)
	require.NoError(t, err)
	assert.Equal(t, ai, string(data))

	assert.False(t, view.Untrusted)

	// A copy carries the document id, so its scripts wait for approval.
	copied := t.TempDir()
	require.NoError(t, os.CopyFS(copied, os.DirFS(root)))
	require.Empty(t, d.loadProject(copied, false))
	s.windows.add("settings-1", projectID(d.app.ProjectRoot()))
	view = s.Load(ctx)
	assert.True(t, view.Untrusted, "a copy at another place must not read the secrets unasked")
	assert.NotEmpty(t, s.SetSecret(ctx, "other", "v"), "adding a secret does not approve the existing ones")
	assert.NotEmpty(t, s.DeleteSecret(ctx, "ai_api_key"), "a copy cannot delete the original's secrets")
	approve = false
	assert.NotEmpty(t, s.TrustSecrets(ctx), "trust needs the user's approval")
	assert.True(t, s.Load(ctx).Untrusted)
	approve = true
	require.Empty(t, s.TrustSecrets(ctx))
	assert.False(t, s.Load(ctx).Untrusted)

	require.Empty(t, s.DeleteSecret(ctx, "ai_api_key"))
	assert.Empty(t, s.Load(ctx).Secrets)
	require.Empty(t, s.SaveFile(ctx, hostconfig.AIFile, ""))
	assert.Empty(t, s.Load(ctx).AI)
}

// Settings opened from one project's window edit that project, not the one
// opened last.
func TestProjectSettings_EditsTheNamedProject(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, root := range []string{first, second} {
		writeTestFile(t, root, "schema.yaml", testSchema)
		writeTestFile(t, root, "data-entry.yaml", testDataEntry)
	}
	d := newTestDesktop(t)
	store, err := newKeychainSecrets(newFakeKeychain())
	require.NoError(t, err)
	d.keychain = store
	require.Empty(t, d.loadProject(first, false))
	firstID := projectID(d.app.ProjectRoot())
	require.Empty(t, d.loadProject(second, true))
	s, err := newProjectSettings(d, nil)
	require.NoError(t, err)
	s.windows.add("settings-first", firstID)
	s.windows.add("settings-second", projectID(d.app.ProjectRoot()))
	firstWin := fromWindow("settings-first")

	require.Empty(t, s.SaveFile(firstWin, hostconfig.AIFile, "base_url: http://localhost:1/v1\nmodel: m\n"))
	assert.NotEmpty(t, s.Load(firstWin).AI)
	assert.Empty(t, s.Load(fromWindow("settings-second")).AI, "the active project is untouched")
	assert.Contains(t, s.SaveFile(firstWin, hostconfig.MailFile, "transport: memory\nfrom: rela@example.com\n"),
		"Reopen", "only the active project can be reopened from here")
}
