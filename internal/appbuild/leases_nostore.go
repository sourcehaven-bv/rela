//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storeLeasesFor returns nil outside the postgres build: those tiers are
// single-process per project, so the KV backend over the node-local cache
// directory is safe. sqlite keeps leases out of rela.db for the reason it
// keeps piles out: the file is shipped to other people.
//
// Nil: always returns a genuinely nil interface, never a typed nil.
func storeLeasesFor(_ store.Store) lease.Store { return nil }
