package appbuild

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// joinRecords returns a record store whose writes join the open store
// transaction view, on the view's own database handle.
type joinRecords func(view store.Store) (records.Store, error)

// txRecordLock is the [records.WriteLock] of a record store kept in the
// entity store's database. It runs an append's owner check and insert in one
// write transaction of the entity store, with the insert on that
// transaction's handle. The store's write lock orders the append against
// entity writes: postgres takes its schema-wide advisory lock, sqlite its
// write mutex and BEGIN IMMEDIATE. An update that closes a ledger period, or
// a delete of the owner, therefore commits before the check, which then sees
// it, or waits until the append has committed.
//
// The owner check reads the owner's raw properties on the transaction view
// ([store.ContextWithTxView], see recordOwners.Writable). Postgres sees the
// row under its default READ COMMITTED isolation, which takes each
// statement's snapshot after the advisory lock was granted.
//
// The fs and memory tiers keep records in files that share no transaction
// with the entity store (see joinRecordsFor), so there the check and the
// insert are not ordered against entity writes.
type txRecordLock struct {
	st   store.Store
	join joinRecords
}

// recordLock returns the write lock for backend over st.
//
// Nil: returned when st has no transaction a record write can join.
func recordLock(st store.Store, backend records.Store) records.WriteLock {
	join := joinRecordsFor(st, backend)
	if join == nil {
		return nil
	}
	return txRecordLock{st: st, join: join}
}

// errRecordsNested is a record write asked for inside a store transaction.
// It would wait for the write lock its own caller holds.
var errRecordsNested = errors.New("appbuild: records: a record write cannot run inside a store transaction")

// Locked implements [records.WriteLock].
func (l txRecordLock) Locked(ctx context.Context, fn func(context.Context, records.Store) error) error {
	if store.InTx(ctx) {
		return errRecordsNested
	}
	return l.st.Tx(ctx, func(view store.Store) error {
		in, err := l.join(view)
		if err != nil {
			return err
		}
		return fn(store.ContextWithTxView(ctx, view), in)
	})
}
