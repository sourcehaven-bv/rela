//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storeRecordsFor returns nil outside the postgres build. The fs, memory
// and desktop tiers use the file backend; the sqlite recipe supplies its
// database backend through backendOverrides.
//
// Nil: always returns a genuinely nil interface, never a typed nil.
func storeRecordsFor(_ store.Store) records.Store { return nil }
