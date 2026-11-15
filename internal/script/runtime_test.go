package script

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// After RequireHostConfig, deps without a host config are refused instead of
// reading the project's .rela directory, which in tenant mode is the
// operator's and shared by every tenant. Deps with one still build.
func TestNewWriterRuntime_RequireHostConfig(t *testing.T) {
	root := t.TempDir()
	require := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	require(os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require(os.WriteFile(filepath.Join(root, ".rela", "secrets.yaml"), []byte("token: operator-token\n"), 0o600))
	deps := lua.WriteDeps{ReadDeps: lua.ReadDeps{ProjectRoot: root, World: store.TrivialScope()}, EntityManager: nopMutator{}}

	rt, err := NewWriterRuntime(deps, "probe.lua", io.Discard)
	require(err)
	rt.Close()

	t.Cleanup(func() { hostConfigRequired.Store(false) })
	RequireHostConfig()

	if rt, err = NewWriterRuntime(deps, "probe.lua", io.Discard); err == nil {
		rt.Close()
		t.Fatal("a runtime without a host config was built after RequireHostConfig")
	}

	deps.Host = hostconfig.Dir("")
	rt, err = NewWriterRuntime(deps, "probe.lua", io.Discard)
	require(err)
	rt.Close()
}
