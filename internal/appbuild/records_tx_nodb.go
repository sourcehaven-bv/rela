//go:build !postgres && !sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// joinRecordsFor returns nil: the filesystem and memory tiers keep records
// in files that have no transaction in common with the entity store.
//
// Nil: always returned.
func joinRecordsFor(store.Store, records.Store) joinRecords { return nil }
