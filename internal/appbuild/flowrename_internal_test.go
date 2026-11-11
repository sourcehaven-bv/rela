package appbuild

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/kvflowrun"
)

// kvRunsWith returns a KV run store holding one run on subject.
func kvRunsWith(t *testing.T, subject string) flowrun.Store {
	t.Helper()
	runs, err := kvflowrun.New(memoryKV(), filelock.ProcessPrivate{})
	require.NoError(t, err)
	_, err = runs.Create(context.Background(), flowrun.NewRun{
		ID: "FR-1", Flow: "onboard", Version: "0123456789abcdef", Definition: []byte("states:\n  open: {}\n"),
		Subject: entity.Ref{ID: subject, Face: entity.ImplicitFace}, At: time.Unix(1_700_000_000, 0).UTC(),
	})
	require.NoError(t, err)
	return runs
}

// failingRuns is a run store whose reads fail.
type failingRuns struct{ flowrun.Store }

func (failingRuns) List(context.Context, flowrun.Query) ([]flowrun.Run, error) {
	return nil, errors.New("run store down")
}

// TestFlowRenameGuard pins the guard's three answers: a held subject is
// refused, a free one admitted, and an unreadable run store refuses.
func TestFlowRenameGuard(t *testing.T) {
	require.Nil(t, flowRenameGuardFor(nil, nil), "no flows: no guard")
	g := flowRenameGuardFor(nil, kvRunsWith(t, "PER-1"))
	require.ErrorIs(t, g.CheckRename(context.Background(), "PER-1"), entitymanager.ErrRenameBlocked)
	require.NoError(t, g.CheckRename(context.Background(), "PER-2"))

	err := flowRenameGuardFor(nil, failingRuns{}).CheckRename(context.Background(), "PER-1")
	require.ErrorIs(t, err, entitymanager.ErrRenameBlocked)
	require.NotContains(t, err.Error(), "run store down", "the cause goes to the log, not the caller")
}

// TestFlowRunAliases_MovesRuns pins the fs backstop: a run left on the old
// id follows the rename.
func TestFlowRunAliases_MovesRuns(t *testing.T) {
	require.Nil(t, newFlowRunAliases(nil))
	runs := kvRunsWith(t, "PER-1")
	require.NoError(t, newFlowRunAliases(runs).EntityRenamed(context.Background(), "PER-1", "PER-9"))
	moved, err := runs.List(context.Background(), flowrun.Query{SubjectID: "PER-9", Limit: 1})
	require.NoError(t, err)
	require.Len(t, moved, 1)
}
