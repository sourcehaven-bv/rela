package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

func TestSecretsCredentialNameCmd(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, project.CacheDir)
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	svc := &readServices{Paths: &project.Context{CacheDir: cacheDir}}

	out, err := captureStdout(t, func() error {
		cmd := &SecretsCredentialNameCmd{}
		return cmd.Run(svc)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(out)
	// The operator pastes this into a unit file, so it must equal exactly what
	// the loader will look for — not merely resemble it.
	if want := secrets.CredentialName(cacheDir); got != want {
		t.Errorf("printed %q, but the loader looks for %q", got, want)
	}
	if !strings.HasPrefix(got, "rela-secrets-") {
		t.Errorf("printed %q, want a rela-secrets- prefix", got)
	}
}

func TestSecretsCredentialNameCmd_UnderivableIsAnError(t *testing.T) {
	// A path that is not a <project>/.rela directory names no project. Printing
	// an empty or invented name would send the operator to a unit-file line
	// that can never match.
	svc := &readServices{Paths: &project.Context{CacheDir: ""}}
	cmd := &SecretsCredentialNameCmd{}
	if err := cmd.Run(svc); err == nil {
		t.Fatal("expected an error for an underivable credential name")
	}
}
