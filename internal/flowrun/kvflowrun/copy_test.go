package kvflowrun

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// failUpdateStore refuses every Update.
type failUpdateStore struct{ *Store }

func (failUpdateStore) Update(context.Context, flowrun.RunID, int64, flowrun.State, time.Time) (flowrun.Run, error) {
	return flowrun.Run{}, errors.New("disk full")
}

// TestCopyRun_RemovesPartialCopy pins that a run whose state step fails is
// not left behind in its start state.
func TestCopyRun_RemovesPartialCopy(t *testing.T) {
	ctx := context.Background()
	src, err := New(newMemKV(t), filelock.ProcessPrivate{})
	require.NoError(t, err)
	run, err := src.Create(ctx, newRun("FR-1"))
	require.NoError(t, err)
	st := run.State
	st.Claim = &flowrun.Claim{Generation: 1, By: "u", At: t0}
	run, err = src.Update(ctx, run.ID, run.Rev, st, t0.Add(time.Minute))
	require.NoError(t, err)

	dst, err := New(newMemKV(t), filelock.ProcessPrivate{})
	require.NoError(t, err)
	err = flowrun.CopyRun(ctx, src, failUpdateStore{dst}, run)
	require.ErrorContains(t, err, "disk full")
	_, err = dst.Get(ctx, "FR-1")
	require.ErrorIs(t, err, flowrun.ErrNotFound)
}
