package kvstate_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

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

// TestEndedRunsAreCapped pins the retention bound. The document is rewritten
// whole on every write, so without it a one-minute task would grow it by 1,440
// runs a day until the next Prune.
func TestEndedRunsAreCapped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kv := newKV(t)
	s, err := kvstate.New(kv)
	require.NoError(t, err)

	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	never := func(int) time.Duration { return time.Minute }
	for i := range 25 {
		at := base.Add(time.Duration(i) * time.Minute)
		loaded, loadErr := s.Load(ctx, []string{"task"})
		require.NoError(t, loadErr)
		id := fmt.Sprintf("r%02d", i)
		require.NoError(t, s.CreateRun(ctx, schedulerstate.Run{
			ID: id, Task: "task", CreatedAt: at, LeaseUntil: at.Add(time.Hour),
		}, loaded["task"].Version))
		_, err = s.FinishRun(ctx, id, schedulerstate.Outcome{At: at}, never)
		require.NoError(t, err)
	}

	data, err := kv.Get(ctx, kvstate.StateKey)
	require.NoError(t, err)
	var doc struct {
		Runs map[string]json.RawMessage `json:"runs"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	require.Len(t, doc.Runs, 10)
	require.Contains(t, doc.Runs, "r24", "the newest runs are the ones kept")
	require.NotContains(t, doc.Runs, "r14")
}
