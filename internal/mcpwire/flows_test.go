//go:build !postgres

package mcpwire

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// flowServices builds the onboarding sample of flowcfg's testdata, with or
// without its flows.yaml.
func flowServices(t *testing.T, withFlows bool) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"entities", "relations", filepath.Join(".rela", "audit")} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	files := map[string]string{"schema.yaml": "metamodel.yaml", "acl.yaml": "acl.yaml"}
	if withFlows {
		files["flows.yaml"] = "flows.yaml"
	}
	for src, dst := range files {
		data, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", src))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, dst), data, 0o600))
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)
	svc, err := appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// TestCurrentFlows_ResolvesPerCall pins that the flow tools follow the
// assembly current returns at each call: after a reload that drops the
// flows, a call answers not found instead of using the old assembly.
func TestCurrentFlows_ResolvesPerCall(t *testing.T) {
	with, without := flowServices(t, true), flowServices(t, false)
	current := with
	cf := currentFlows(func() *appbuild.Services { return current })

	// Without an identity the flow control fails closed, which shows the
	// call reached it.
	_, err := cf.Tasks(context.Background(), "", 1, 10)
	require.Error(t, err)
	require.NotErrorIs(t, err, flowview.ErrNotFound)

	current = without
	_, err = cf.Tasks(context.Background(), "", 1, 10)
	require.ErrorIs(t, err, flowview.ErrNotFound)
	_, err = cf.Run(context.Background(), "FR-1")
	require.ErrorIs(t, err, flowview.ErrNotFound)
	_, err = cf.Complete(context.Background(), "FR-1", 1, nil)
	require.ErrorIs(t, err, flowview.ErrNotFound)

	require.NotNil(t, CurrentFlows(func() *appbuild.Services { return without }))
}
