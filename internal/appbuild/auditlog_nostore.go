//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storeAuditLogFor returns nil outside the postgres build: the fs, memory
// and desktop tiers keep the JSONL sink, and sqlite supplies its log as a
// recipe override.
func storeAuditLogFor(store.Store) *sqlaudit.Log { return nil }
