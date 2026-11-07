package appbuild

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// deletedSecret is a value that must never reach a job payload in clear.
const deletedSecret = "hidden-value-7f3a"

func secretNote() *entity.Entity {
	e := deletedNote("note")
	e.Properties["secret"] = deletedSecret
	return e
}

// queuedDeleted deletes a note with a queue in place and returns the jobs
// service, the runner, and the one job it queued.
func queuedDeleted(t *testing.T, live liveEntities) (*automationJobs, *fakeJobRunner, jobs.Job) {
	t.Helper()
	q := &dupQueue{}
	r := &fakeJobRunner{}
	var raw rawEntityLister
	if live != nil {
		raw = live
	}
	a := newDeletedJobs(t, q, r, raw)
	require.NoError(t, a.EntityStateDeleted(context.Background(), secretNote()))
	require.Len(t, q.jobs, 1)
	return a, r, q.jobs[0]
}

// sealedOpens reports whether the job's sealed state can still be read.
func sealedOpens(t *testing.T, a *automationJobs, j jobs.Job) bool {
	t.Helper()
	p, err := payloadFromMap(j.Payload)
	require.NoError(t, err)
	_, err = a.deleted.open(context.Background(), p.ID, p.Face, p.Handle, p.Sealed)
	if errors.Is(err, errDeletedStateErased) {
		return false
	}
	require.NoError(t, err)
	return true
}

// TestDeletedState_PayloadHoldsNoClearState: the job payload carries the
// deleted state only sealed, so the job table never holds its values.
func TestDeletedState_PayloadHoldsNoClearState(t *testing.T) {
	a, _, j := queuedDeleted(t, nil)
	b, err := json.Marshal(j.Payload)
	require.NoError(t, err)
	require.NotContains(t, string(b), deletedSecret)
	require.NotContains(t, j.Payload, "last")
	require.True(t, sealedOpens(t, a, j))
}

// TestDeletedState_ErasedWhenJobEnds: the key that opens the sealed state is
// erased once no further delivery will follow, and kept for a retry.
func TestDeletedState_ErasedWhenJobEnds(t *testing.T) {
	tests := []struct {
		name      string
		live      liveEntities
		runErr    error
		attempt   int
		retry     jobs.Retry
		wantRun   bool
		wantError bool
		wantOpens bool
	}{
		{name: "completed", attempt: 1, retry: jobs.RetryBounded, wantRun: true},
		{name: "dropped: face stored again", live: liveEntities{deletedNote("note")}, attempt: 1,
			retry: jobs.RetryBounded},
		{name: "failed, retry follows", runErr: errors.New("boom"), attempt: 1, retry: jobs.RetryBounded,
			wantRun: true, wantError: true, wantOpens: true},
		{name: "failed on the final attempt", runErr: errors.New("boom"), attempt: 1, retry: jobs.RetryNever,
			wantRun: true, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, r, j := queuedDeleted(t, tc.live)
			r.err = tc.runErr
			j.Attempt, j.Retry = tc.attempt, tc.retry
			err := a.handle(context.Background(), j)
			require.Equal(t, tc.wantError, err != nil, "handle error: %v", err)
			require.Equal(t, tc.wantRun, r.count() == 1)
			if tc.wantRun {
				require.NotContains(t, r.seen[0].Properties, "secret", "a hidden property reached the script")
			}
			require.Equal(t, tc.wantOpens, sealedOpens(t, a, j))
		})
	}
}

// TestDeletedState_ErasedOnPurge: a purge of the entity, by the soft-delete
// purge or by `rela history-purge`, erases the keys of its pending jobs, so
// they end without reading the state. Other entities keep theirs.
func TestDeletedState_ErasedOnPurge(t *testing.T) {
	tests := []struct {
		name  string
		purge func(context.Context, *automationJobs, string) error
	}{
		{"soft-delete purge", func(ctx context.Context, a *automationJobs, id string) error {
			return a.EntityPurged(ctx, id)
		}},
		{"history-purge", func(ctx context.Context, a *automationJobs, id string) error {
			return PurgeDeletedJobStates(ctx, a.kv, a.meta, id)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			a, r, j := queuedDeleted(t, nil)
			other := secretNote()
			other.ID = "NOTE-10"
			handle, sealed, err := a.deleted.seal(ctx, other)
			require.NoError(t, err)

			require.NoError(t, tc.purge(ctx, a, "NOTE-9"))
			require.False(t, sealedOpens(t, a, j))
			require.NoError(t, a.handle(ctx, j), "an erased job ends without a retry")
			require.Zero(t, r.count())

			_, err = a.deleted.open(ctx, other.ID, other.Face, handle, sealed)
			require.NoError(t, err, "another entity's state was erased")
		})
	}
}

// TestDeletedState_EnqueueFailureErases: a refused enqueue leaves no key.
func TestDeletedState_EnqueueFailureErases(t *testing.T) {
	a := newDeletedJobs(t, refusingQueue{}, &fakeJobRunner{}, nil)
	require.Error(t, a.EntityStateDeleted(context.Background(), secretNote()))
	handles, err := a.deleted.readIndex(context.Background(), "NOTE-9")
	require.NoError(t, err)
	require.Empty(t, handles)
	kv, ok := a.kv.(*mapKV)
	require.True(t, ok)
	require.Empty(t, kv.m)
}

type refusingQueue struct{}

func (refusingQueue) Register(jobs.Kind, jobs.Handler) error { return nil }
func (refusingQueue) Enqueue(context.Context, jobs.Job) error {
	return errors.New("refused")
}

// TestDeletedState_PurgeWithoutIndexEntry: a state whose handle never
// reached the index, as when two processes update it at once, is still
// unreadable after a purge, while a state sealed after the purge is not.
func TestDeletedState_PurgeWithoutIndexEntry(t *testing.T) {
	ctx := context.Background()
	a, r, j := queuedDeleted(t, nil)
	require.NoError(t, a.kv.Delete(ctx, deletedStateDir("NOTE-9")+deletedStateIndex))

	require.NoError(t, a.EntityPurged(ctx, "NOTE-9"))
	require.False(t, sealedOpens(t, a, j))
	require.NoError(t, a.handle(ctx, j))
	require.Zero(t, r.count())

	later := secretNote()
	handle, sealed, err := a.deleted.seal(ctx, later)
	require.NoError(t, err)
	_, err = a.deleted.open(ctx, later.ID, later.Face, handle, sealed)
	require.NoError(t, err, "a delete after the purge must stay readable")
}
