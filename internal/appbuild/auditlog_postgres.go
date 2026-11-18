//go:build postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/audit/pgaudit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// storeAuditLogFor returns the audit log in the store's schema, so each
// tenant has one chain shared by every process of the deployment. Nil when
// st is not a pgstore (a test assembling over memstore).
func storeAuditLogFor(st store.Store) *sqlaudit.Log {
	db := pgstore.HandleFor(st)
	if db == nil {
		return nil
	}
	log, err := pgaudit.New(db)
	if err != nil { // coverage-ignore: unreachable: db is non-nil here
		panic(err)
	}
	return log
}
