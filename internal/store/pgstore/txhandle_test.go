//go:build postgres

package pgstore_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestTxHandleFor_OnlyInsideTx pins that the pool is never handed out as a
// transaction: a join on it would commit independently of the store's write.
func TestTxHandleFor_OnlyInsideTx(t *testing.T) {
	st := newXrefStore(t)
	if got := pgstore.TxHandleFor(st); got != nil {
		t.Fatalf("TxHandleFor(store) = %T, want nil", got)
	}
	err := st.Tx(t.Context(), func(view store.Store) error {
		if pgstore.TxHandleFor(view) == nil {
			t.Error("TxHandleFor(view) = nil inside Tx")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
