package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// withSQLAudit returns base with its audit sink replaced by one over the
// store's database, and that sink, when the backend keeps an audit log in
// its database (postgres, sqlite). Otherwise it returns base unchanged and
// a nil sink: the fs and memory tiers keep Config.Audit, the JSONL sink in
// production.
//
// A re-assembly (see [SharedBase.ForReassembly]) reuses its predecessor's
// sink, so a schema reload neither falls back to Config.Audit nor closes the
// sink that in-flight writes of the predecessor still use. The caller
// passes its assembly error to abandon, which closes the sink when it was
// created here and assembly failed.
//
// The configured sink is replaced rather than joined: on a database tier the
// SQL log is the one place a record is kept, so every process of a
// deployment appends to one chain. base itself is not changed; a copy
// carries the new sink.
func withSQLAudit(
	base *SharedBase, st store.Store, o backendOverrides,
) (next *SharedBase, abandon func(assemblyErr error), err error) {
	keep := func(error) {}
	sink := base.sqlAudit
	if sink == nil {
		log := o.auditLog
		if log == nil {
			log = storeAuditLogFor(st)
		}
		if log == nil {
			return base, keep, nil
		}
		if sink, err = sqlaudit.NewSink(log); err != nil { // coverage-ignore: unreachable: log is non-nil
			return nil, keep, err
		}
		created := sink
		keep = func(assemblyErr error) {
			if assemblyErr != nil {
				_ = created.Close()
			}
		}
	}
	withSink := *base
	withSink.cfg.Audit = sink
	if join := joinAuditFor(st); join != nil {
		withSink.cfg.Audit = txAuditSink{Sink: sink, join: join}
	}
	withSink.sqlAudit = sink
	return &withSink, keep, nil
}

// joinAudit returns the audit log of the open store transaction that view
// belongs to. It fails when view is not a transaction view of the database
// store, which rolls the write back.
type joinAudit func(view store.Store) (*sqlaudit.Log, error)

// txAuditSink is the database audit sink of a store whose transactions the
// log can join. It is an [entitymanager.TxAudit]: the entity manager appends
// the record of an entity or relation write in the write's own transaction,
// so a failed append rolls the write back (DEC-U78WD7Y). Records of
// everything else, such as a denied write, go through the embedded sink's
// queue.
type txAuditSink struct {
	*sqlaudit.Sink
	join joinAudit
}

// RecordTx appends recs in the transaction of view.
func (s txAuditSink) RecordTx(ctx context.Context, view store.Store, recs []audit.Record) error {
	log, err := s.join(view)
	if err != nil {
		return err
	}
	return log.AppendAll(ctx, recs)
}

var _ entitymanager.TxAudit = txAuditSink{}
