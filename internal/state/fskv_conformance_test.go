package state_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/state/statetest"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestFSKV_Conformance(t *testing.T) {
	statetest.RunAll(t, func(tb testing.TB) state.KV {
		tb.Helper()
		mem := storage.NewMemFS()
		if err := mem.MkdirAll("/root", 0o755); err != nil {
			tb.Fatalf("mkdir root: %v", err)
		}
		rfs, err := storage.NewRootedFS(mem, "/root")
		if err != nil {
			tb.Fatalf("NewRootedFS: %v", err)
		}
		return state.NewFSKV(rfs)
	})
}
