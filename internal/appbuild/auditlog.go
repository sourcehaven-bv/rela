package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
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
	withSink.sqlAudit = sink
	return &withSink, keep, nil
}
