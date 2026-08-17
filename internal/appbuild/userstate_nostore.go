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
func storeUserStateFor(_ store.Store) userstate.Store { return nil }
