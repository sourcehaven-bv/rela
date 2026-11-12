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

// storeHintsFor returns nil outside the postgres build: the file store's
// journal serves the fs tiers (see hintEndpointFor).
//
// Nil: always returned.
func storeHintsFor(_ channels.Store, _ store.Store) hintEndpoint { return nil }

// storeLeaderLockFor returns nil outside the postgres build: the other
// tiers take a worker lock on a lock file.
//
// Nil: always returned.
func storeLeaderLockFor(_ store.Store, _ string) leaderLock { return nil }

// storeLockRoom has nothing to check outside the postgres build: a lock
// file pins no connection.
func storeLockRoom(_ store.Store, _ int) error { return nil }
