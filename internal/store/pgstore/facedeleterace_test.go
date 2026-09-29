package pgstore_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestDeleteEntityState_RacingFaceCreateKeepsAttachments pins that deleting
// the last face serializes with a create of a new face on the family lock,
// before the delete reads anything, and that the attachments survive when the
// create wins (BUG-J3PBFN).
//
// The row lock this replaced was on the bare row, which a faced type does not
// store, so it locked nothing. The delete was still serialized in this
// interleaving, but only by accident: the create's FOR SHARE on the sibling
// rows blocked the delete's row DELETE. The family lock makes the guarantee
// explicit and the same one DeleteEntity and CreateEntity take.
//
// Deterministic, unlike TestDeleteEntity_RacingStateCreateIsAtomicPerFamily:
// the create runs in a transaction that holds the family lock until the test
// releases it, and the test waits until the delete is observed queued on that
// lock. With the bare-row lock the delete queues on a row lock instead, and
// the test fails at its deadline.
func TestDeleteEntityState_RacingFaceCreateKeepsAttachments(t *testing.T) {
	// Four connections: the create transaction and the delete each hold one
	// while the test polls pg_locks on a third.
	pool := newScopedPoolSized(t, 4)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	draft := entity.New("POL-1", "policy")
	draft.Face = "draft"
	require.NoError(t, s.CreateEntity(ctx, draft))
	require.NoError(t, s.AttachFile(ctx, "POL-1", "file", "a.txt", bytes.NewReader([]byte("a"))))

	created := make(chan struct{})
	release := make(chan struct{})
	txDone := make(chan error, 1)
	go func() {
		txDone <- s.Tx(ctx, func(tx store.Store) error {
			published := entity.New("POL-1", "policy")
			published.Face = "published"
			if cErr := tx.CreateEntity(ctx, published); cErr != nil {
				return cErr
			}
			close(created)
			<-release
			return nil
		})
	}()
	select {
	case <-created:
	case cErr := <-txDone:
		t.Fatalf("create transaction ended early: %v", cErr)
	}

	delDone := make(chan error, 1)
	go func() {
		_, dErr := s.DeleteEntityState(ctx, "POL-1", "draft")
		delDone <- dErr
	}()

	// Wait until the delete is queued on the family lock, or has finished
	// without queuing.
	deadline := time.Now().Add(10 * time.Second)
	for waiting := false; !waiting; {
		select {
		case dErr := <-delDone:
			close(release)
			require.NoError(t, <-txDone)
			require.NoError(t, dErr)
			t.Fatal("DeleteEntityState finished while a create of the same family held the family lock")
		default:
		}
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) > 0 FROM pg_locks WHERE locktype = 'advisory' AND classid = $1 AND NOT granted`,
			pgstore.FamilyAdvisoryLockKeyForTest).Scan(&waiting))
		if time.Now().After(deadline) {
			close(release)
			t.Fatal("DeleteEntityState did not queue on the family lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	close(release)
	require.NoError(t, <-txDone)
	require.NoError(t, <-delDone)

	infos, err := s.ListAttachments(ctx, "POL-1")
	require.NoError(t, err)
	require.Len(t, infos, 1, "the surviving published face lost the entity's attachments")
	_, err = s.GetEntityState(ctx, "POL-1", "published")
	require.NoError(t, err)
}
