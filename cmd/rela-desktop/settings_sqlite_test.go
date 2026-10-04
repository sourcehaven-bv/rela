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
	s := &ProjectSettings{d: d}

	require.Empty(t, s.SetSecret("ai_api_key", "sk-test"))
	assert.NotEmpty(t, s.SetSecret("bad name", "x"))
	view := s.Load()
	require.Empty(t, view.Error)
	assert.Equal(t, []string{"ai_api_key"}, view.Secrets)
	for item := range kc.items {
		assert.NotContains(t, item, root, "keychain items are keyed by document id, not path")
	}

	ai := "base_url: http://localhost:1/v1\nmodel: m\n"
	require.Empty(t, s.SaveFile(hostconfig.AIFile, ai))
	assert.NotEmpty(t, s.SaveFile(hostconfig.AIFile, "model: m\n"), "an invalid ai.yaml is refused")
	assert.NotEmpty(t, s.SaveFile("secrets.yaml", "x: y\n"))
	require.Empty(t, s.SaveFile(hostconfig.MailFile, "transport: memory\nfrom: rela@example.com\n"))

	_, err = os.Stat(filepath.Join(root, ".rela", hostconfig.AIFile))
	assert.True(t, os.IsNotExist(err), "settings are not written to .rela")

	require.Empty(t, d.loadProject(root, false))
	view = s.Load()
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
	view = s.Load()
	assert.True(t, view.Untrusted, "a copy at another place must not read the secrets unasked")
	assert.NotEmpty(t, s.SetSecret("other", "v"), "adding a secret does not approve the existing ones")
	require.Empty(t, s.TrustSecrets())
	assert.False(t, s.Load().Untrusted)

	require.Empty(t, s.DeleteSecret("ai_api_key"))
	assert.Empty(t, s.Load().Secrets)
	require.Empty(t, s.SaveFile(hostconfig.AIFile, ""))
	assert.Empty(t, s.Load().AI)
}
