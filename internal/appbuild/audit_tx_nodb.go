//go:build !postgres && !sqlite

package appbuild

import "github.com/Sourcehaven-BV/rela/internal/store"

// joinAuditFor returns nil: the fs and memory tiers keep no audit log in a
// database, so their writes are audited best effort.
func joinAuditFor(store.Store) joinAudit { return nil }
