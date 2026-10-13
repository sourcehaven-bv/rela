//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storeChannelsFor returns nil outside the postgres build. The fs, memory
// and desktop tiers use the file backend; the sqlite recipe supplies its
// database backend through backendOverrides.
//
// Nil: always returns a genuinely nil interface, never a typed nil.
func storeChannelsFor(_ store.Store) channels.Store { return nil }
