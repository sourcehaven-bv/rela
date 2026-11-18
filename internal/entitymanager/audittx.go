package entitymanager

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TxAudit is the capability of a [Deps.Audit] whose log is kept in the
// store's database (postgres, sqlite). RecordTx appends recs, in order, in
// the store transaction that view belongs to, so the records commit or roll
// back with the write they describe. An error rolls the write back
// (DEC-U78WD7Y).
//
// The manager type-asserts [Deps.Audit] to it. A sink without it (the JSONL
// sink of the fs tier, the memory sink in tests) receives the record
// through [audit.Audit.Record] instead, best effort, and the write stands
// whatever happens to the record.
type TxAudit interface {
	RecordTx(ctx context.Context, view store.Store, recs []audit.Record) error
}

// errAuditOutsideTx is the record of a write that was not handed the store
// the write went to. It is a bug in this package.
var errAuditOutsideTx = errors.New("entitymanager: audit record of a write outside its store transaction")

// auditsInTx reports whether deps records writes inside their transaction.
// A write path then opens a store transaction even when nothing else needs
// one, so the record joins it.
func auditsInTx(deps Deps) bool {
	_, ok := deps.Audit.(TxAudit)
	return ok
}

// emitWrite records rec, the audit record of a write to st.
//
// With a [TxAudit] sink, st must be the view of the write's open store
// transaction; the append joins it, and its error is returned so that the
// caller fails the write and the transaction rolls back. The sink refuses a
// store that is not a transaction view. Otherwise rec goes to the
// best-effort sink and emitWrite returns nil.
//
// Nil: st rejected with a TxAudit sink; ignored otherwise.
func emitWrite(ctx context.Context, deps Deps, st store.Store, rec audit.Record) error {
	tx, ok := deps.Audit.(TxAudit)
	if !ok {
		if p, held := ctx.Value(heldAuditKey{}).(*heldAudit); held {
			p.recs = append(p.recs, rec)
			return nil
		}
		deps.Audit.Record(rec)
		return nil
	}
	if st == nil {
		return errAuditOutsideTx
	}
	if b, ok := ctx.Value(txBatchKey{}).(*txBatch); ok && b.view == st {
		b.recs = append(b.recs, rec)
		return nil
	}
	return tx.RecordTx(ctx, st, []audit.Record{rec})
}

// auditTx runs fn in a store transaction when deps audits writes in one or
// need is true, and on deps.Store directly otherwise. Every write path that
// audits opens its transaction here, so that a best-effort sink receives
// the records only once the transaction commits ([heldAudit]). fn receives the
// context and the store to write and audit on: inside a transaction the
// context carries the view ([store.ContextWithTxView]). A ctx that already
// carries a view joins that transaction rather than open a second one, which
// on postgres would wait for the lock its own caller holds.
func auditTx(ctx context.Context, deps Deps, need bool, fn func(context.Context, store.Store) error) error {
	if view := store.TxView(ctx); view != nil {
		return fn(ctx, view)
	}
	if !need && !auditsInTx(deps) {
		return fn(ctx, deps.Store)
	}
	if tx, ok := deps.Audit.(TxAudit); ok {
		b := &txBatch{}
		batchCtx := context.WithValue(ctx, txBatchKey{}, b)
		return deps.Store.Tx(ctx, func(view store.Store) error {
			b.view = view
			if err := fn(store.ContextWithTxView(batchCtx, view), view); err != nil {
				return err
			}
			if len(b.recs) == 0 {
				return nil
			}
			return tx.RecordTx(ctx, view, b.recs)
		})
	}
	held := &heldAudit{}
	heldCtx := context.WithValue(ctx, heldAuditKey{}, held)
	err := deps.Store.Tx(heldCtx, func(view store.Store) error {
		return fn(store.ContextWithTxView(heldCtx, view), view)
	})
	if err != nil && store.TxRollsBack(deps.Store) {
		return err
	}
	for _, rec := range held.recs {
		deps.Audit.Record(rec)
	}
	return err
}

// heldAudit holds the records a best-effort sink receives inside a
// transaction [auditTx] opened, until the transaction commits. A record
// must not describe a write that was rolled back, and a sink outside the
// database cannot roll back with it. A failed transaction drops what it
// held on a store that rolls back. On one that cannot (fs, memory), each
// held record describes a write that stayed, so it is delivered.
type heldAudit struct{ recs []audit.Record }

type heldAuditKey struct{}

// txBatch collects the records a [TxAudit] sink receives inside a
// transaction [auditTx] opened, and appends them in one call before the
// transaction commits. A write that records many rows (a cascade delete, a
// renumber, a batch) then reads the chain head and takes the chain lock
// once, not once per row, while it holds the store's write lock. A record
// for another store than the batch's view is appended at once.
type txBatch struct {
	view store.Store
	recs []audit.Record
}

type txBatchKey struct{}
