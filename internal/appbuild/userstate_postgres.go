//go:build postgres

package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// storeUserStateFor returns the pgstore next-action state backend, sharing the
// store's pool. Mirrors versionServiceFor: a build-tagged resolver with a
// concrete type assertion, so the non-postgres builds neither know nor link
// this implementation.
//
// This backend is preferred over the state.KV one because it is the only one
// safe for a multi-process deployment: the KV backend rewrites one JSON
// document per write, so two servers sharing a project directory silently lose
// each other's snoozes.
//
// Returns a genuinely nil interface on failure (not a typed nil) so the
// caller's nil-check behaves, falling back to the KV backend rather than
// handing out a broken handle.
func storeUserStateFor(st store.Store) userstate.Store {
	s, ok := st.(*pgstore.Store)
	if !ok {
		return nil
	}
	us, err := s.UserState()
	if err != nil {
		slog.Warn("next-action state: pgstore backend unavailable", "error", err)
		return nil
	}
	return us
}
