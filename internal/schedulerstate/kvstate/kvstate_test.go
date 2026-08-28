package kvstate_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate/kvstate"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate/schedulerstatetest"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newKV(t *testing.T) state.KV {
	t.Helper()
	mem := storage.NewMemFS()
	require.NoError(t, mem.MkdirAll("/root", 0o755))
	rfs, err := storage.NewRootedFS(mem, "/root")
	require.NoError(t, err)
	return state.NewFSKV(rfs)
}

// TestConformance runs the shared contract against the KV backend.
func TestConformance(t *testing.T) {
	t.Parallel()

	schedulerstatetest.RunAll(t, func(t *testing.T) schedulerstate.Store {
		t.Helper()
		s, err := kvstate.New(newKV(t))
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Close() })
		return s
	})
}

func TestNew_RejectsNilKV(t *testing.T) {
	t.Parallel()

	_, err := kvstate.New(nil)
	require.Error(t, err)
}
