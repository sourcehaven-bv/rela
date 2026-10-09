package appbuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

// cmdexec names the token key variable itself, because it may not import
// tokenstore. This keeps the two names the same.
func TestCmdexecWithholdsTheTokenKey(t *testing.T) {
	t.Setenv(tokenstore.KeyEnv, "k")
	for _, kv := range cmdexec.Environ() {
		if strings.HasPrefix(kv, tokenstore.KeyEnv+"=") {
			t.Fatalf("cmdexec.Environ passes %s to child processes", tokenstore.KeyEnv)
		}
	}
}

// An invalid connections.yaml fails assembly on every backend, instead of
// leaving every connector without tokens behind a log line (TKT-01KZSO).
func TestDiscover_RefusesInvalidConnections(t *testing.T) {
	root := writeMinimalProject(t)
	if err := os.WriteFile(filepath.Join(root, "connections.yaml"),
		[]byte("connections:\n  svc:\n    tokn_url: https://auth.example.com/token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := discover(t, root)
	if err == nil {
		_ = svc.Close()
		t.Fatal("Discover accepted an invalid connections.yaml")
	}
	if !strings.Contains(err.Error(), "connections.yaml") {
		t.Fatalf("err = %v, want one naming connections.yaml", err)
	}
}
