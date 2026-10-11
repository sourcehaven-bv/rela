//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storePilesFor returns nil outside the postgres build: those tiers are
// single-process, so the KV backend over the node-local cache directory is
// safe. The sqlite build deliberately does not keep piles in rela.db, because
// that file is shipped to other people and a pile is personal.
//
// Nil: always returns a genuinely nil interface, never a typed nil.
func storePilesFor(_ store.Store) piles.Store { return nil }
