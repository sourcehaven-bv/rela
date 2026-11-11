//go:build sqlite || postgres

package appbuild_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// writeLockWait bounds the wait for a goroutine to block on the write lock.
const writeLockWait = 30 * time.Second

// failIfFinished fails the test when op has already finished: it was to wait
// for the write lock.
func failIfFinished(t *testing.T, op <-chan error) {
	t.Helper()
	select {
	case err := <-op:
		t.Fatalf("finished without waiting for the write lock: %v", err)
	default:
	}
}

// raceRun is a run of onboarding on PER-1 in its initial state, collect,
// where outside_edits denies documents. The ceiling reads only the run
// store, so the run needs no header.
func raceRun() flowrun.NewRun {
	return flowrun.NewRun{
		ID: "FLR-RACE", Flow: "onboarding", Version: "00000000000000ee", Definition: []byte("race"),
		Subject: entity.Ref{ID: "PER-1", Face: entity.ImplicitFace}, Singleton: true, At: time.Now().UTC(),
	}
}

// holdWriteLock opens a write transaction on svc's store and returns once
// it is open. The transaction runs inside and commits when the caller calls
// finish, or when the test ends. done reports the transaction's result.
func holdWriteLock(t *testing.T, svc *appbuild.Services, inside func(view store.Store) error) (
	finish func(), done <-chan error,
) {
	t.Helper()
	opened := make(chan struct{})
	goCh := make(chan struct{})
	res := make(chan error, 1)
	go func() {
		res <- svc.Store().Tx(context.Background(), func(view store.Store) error {
			close(opened)
			<-goCh
			return inside(view)
		})
	}()
	<-opened
	var once sync.Once
	finish = func() { once.Do(func() { close(goCh) }) }
	// Runs before the services close, which would wait for the open
	// transaction.
	t.Cleanup(finish)
	return finish, res
}

// TestOutsideEdits_RunStartedDuringPatchFailsClosed pins the race closed on
// the database tiers (TKT-XI7IDBS). A gated patch checks the ceiling, finds
// no run and waits for the store's write lock. A run start commits under
// that lock first. The patch then checks again inside its transaction, sees
// the run and is refused, instead of writing a property the run denies.
func TestOutsideEdits_RunStartedDuringPatchFailsClosed(t *testing.T) {
	svc, _ := outsideEditsProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	mgr := entitymanager.FieldGated(svc.EntityManager())
	dave := flowUser(t, svc, "dave")

	finish, held := holdWriteLock(t, svc, func(view store.Store) error {
		runs, err := appbuild.JoinedFlowRuns(svc, view)
		if err != nil {
			return err
		}
		_, err = runs.Create(context.Background(), raceRun())
		return err
	})
	patched := make(chan error, 1)
	go func() {
		_, err := mgr.PatchEntity(dave, "PER-1", entity.Patch{Properties: map[string]any{"documents": true}})
		patched <- err
	}()
	waitForWriteLockWaiter(t, svc, patched, "entitymanager.(*Manager).PatchEntity")
	finish()
	require.NoError(t, <-held)
	requireFlowReadOnly(t, <-patched, "documents")
}

// TestOutsideEdits_RunWriteTakesWriteLock pins the other half: the engine's
// run writes wait for the store's write lock, so a run cannot commit inside
// a field write's transaction, between its check and its write.
func TestOutsideEdits_RunWriteTakesWriteLock(t *testing.T) {
	svc, _ := outsideEditsProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	runs := appbuild.LockedFlowRuns(svc)
	require.NotNil(t, runs)

	finish, held := holdWriteLock(t, svc, func(store.Store) error { return nil })
	created := make(chan error, 1)
	go func() {
		_, err := runs.Create(context.Background(), raceRun())
		created <- err
	}()
	waitForWriteLockWaiter(t, svc, created, "appbuild.txFlowRuns.Create")
	finish()
	require.NoError(t, <-held)
	require.NoError(t, <-created)

	_, err := entitymanager.FieldGated(svc.EntityManager()).PatchEntity(flowUser(t, svc, "dave"), "PER-1",
		entity.Patch{Properties: map[string]any{"documents": true}})
	requireFlowReadOnly(t, err, "documents")
}
