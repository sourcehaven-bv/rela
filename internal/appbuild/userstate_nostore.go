//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// storeUserStateFor returns nil in non-postgres builds: no other store carries
// its own next-action state, so those builds use the state.KV backend.
//
// Returning a GENUINELY nil interface (not a typed nil) is load-bearing — the
// caller nil-checks this to decide whether to fall back, and a typed nil would
// defeat that check. Same contract as versionServiceFor.
// The sqlite build inherits this too (TKT-L1A3PH): the KV backend is unsafe
// only for MULTI-process deployments, because it rewrites one JSON document per
// write and two servers would clobber each other's snoozes. sqlitestore refuses
// a second process at Open, so the hazard cannot arise.
func storeUserStateFor(_ store.Store) userstate.Store { return nil }
