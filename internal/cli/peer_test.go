package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/peerkey"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

func TestPeerKeyCreate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &PeerKeyCreateCmd{Name: "acme"}
	require.NoError(t, cmd.run(bytes.NewReader(bytes.Repeat([]byte{7}, 32)), &stdout, &stderr))

	key := strings.TrimSpace(stdout.String())
	require.True(t, peerkey.IsPeerKey(key), key)
	digest := peerkey.Digest(key)
	require.Contains(t, stderr.String(), "peer_key.acme: "+digest+"\n")
	require.Contains(t, stderr.String(), "peer:acme")
	// The key goes to stdout only.
	require.NotContains(t, stderr.String(), key)

	// The printed line is one the source accepts.
	ring, err := peerkey.NewKeyring(map[string]string{"acme": digest})
	require.NoError(t, err)
	name, err := ring.Verify(key)
	require.NoError(t, err)
	require.Equal(t, "acme", name)

	for _, bad := range []string{"", "Acme", "peer:acme", "-x", strings.Repeat("a", 64)} {
		stdout.Reset()
		err := (&PeerKeyCreateCmd{Name: bad}).run(bytes.NewReader(make([]byte, 32)), &stdout, &stderr)
		require.ErrorContains(t, err, "invalid peer name", bad)
		require.Empty(t, stdout.String())
	}

	// A short random source is an error, not a weak key.
	stdout.Reset()
	require.Error(t, cmd.run(bytes.NewReader(make([]byte, 3)), &stdout, &stderr))
	require.Empty(t, stdout.String())
}

func TestReconcileDiff_TokenSecret(t *testing.T) {
	srv := oneTicketRemote(t, "Bearer relapeer_from-secret")
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	relaDir := filepath.Join(t.TempDir(), ".rela")
	require.NoError(t, os.MkdirAll(relaDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(relaDir, secrets.ConfigFile), []byte(
		"peer_credential.source: relapeer_from-secret\ntoken_key: k\npeer_key.acme: sha256:00\n"+
			"overrides:\n  \"\":\n    other: x\n"), 0o600))
	s := localServices(t)
	s.Secret = func(name string) (string, error) { return globalSecret(relaDir, name) }
	run := func(name string) error {
		_, err := runReconcile(t, s, "reconcile", "diff", "--remote", srv.URL, "--token-secret", name)
		return err
	}

	require.NoError(t, run("peer_credential.source"))

	// The credential flags exclude each other.
	_, err := runReconcile(t, s, "reconcile", "diff", "--remote", srv.URL,
		"--token-secret", "peer_credential.source", "--token-env", "X")
	require.ErrorContains(t, err, "can't be used together")

	require.ErrorContains(t, run("missing"), `--token-secret missing: no secret "missing"`)
	// A per-script override never supplies the credential.
	require.ErrorContains(t, run("other"), `no secret "other"`)

	// The server's own secrets are no credential for a remote.
	for _, name := range []string{secrets.TokenKey, "peer_key.acme"} {
		require.ErrorContains(t, run(name), "not a credential for a remote", name)
	}

	s.Secret = nil
	require.ErrorContains(t, run("peer_credential.source"), "this project has no secrets")
}
