//go:build postgres

package appbuild

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// The state-store half of TKT-L3FNEN AC-2, split out of
// backendneutral_postgres_test.go when the VERSION half became shared with the
// sqlite build (TKT-4NU9ZD).
//
// It stays postgres-only because rawStateStore is: the sqlite build keeps the
// filesystem KV on purpose (TKT-L1A3PH), so there is no second implementation
// for it to be neutral about.
//
// This file must NOT import internal/store/pgstore — same claim, same reason.

// pgNeutralBase is a store with no capabilities at all.
func pgNeutralBase(t *testing.T) store.Store {
	t.Helper()
	m := memstore.New()
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// neutralStateStore satisfies the wiring's rawStateStore structurally, the way
// a non-pgstore backend would: three methods, no state.KV import (a store may
// not depend on that application package).
type neutralStateStore struct{}

func (neutralStateStore) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (neutralStateStore) Put(context.Context, string, []byte) error   { return nil }
func (neutralStateStore) Delete(context.Context, string) error        { return nil }

// TestRawStateStoreIsSatisfiableWithoutPgstore covers the state half, which the
// ticket notes was left out of TKT-415WA7 entirely.
func TestRawStateStoreIsSatisfiableWithoutPgstore(t *testing.T) {
	var _ rawStateStore = neutralStateStore{}

	// And a store with no state capability must fall through to the FSKV, not
	// hand back a non-nil interface wrapping nothing.
	if got := stateKVFor(pgNeutralBase(t)); got != nil {
		t.Errorf("stateKVFor = %#v, want untyped nil so the FSKV fallback engages", got)
	}
}
