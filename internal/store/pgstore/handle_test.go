//go:build postgres

package pgstore

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stubDB is a database handle no test calls.
type stubDB struct{ DBTX }

// decorated wraps a store, as the storetest recorders do.
type decorated struct{ store.Store }

func (d decorated) Unwrap() store.Store { return d.Store }

// TestHandleFor_ThroughDecorators pins that a pgstore behind decorators still
// hands out its handle, so a recorder-wrapped store keeps the database piles
// and state backends rather than falling back to memory.
func TestHandleFor_ThroughDecorators(t *testing.T) {
	db := &stubDB{}
	pg := &Store{db: db}
	wrapped := decorated{decorated{pg}}
	if got := HandleFor(wrapped); got != DBTX(db) {
		t.Errorf("HandleFor(decorated pgstore) = %v, want the store's handle", got)
	}
	if got := StateStoreFor(wrapped); got == nil || got.db != DBTX(db) {
		t.Errorf("StateStoreFor(decorated pgstore) = %v, want a state store on the store's handle", got)
	}
	if got := HandleFor(decorated{}); got != nil {
		t.Errorf("HandleFor(decorated nil) = %v, want nil", got)
	}
}
