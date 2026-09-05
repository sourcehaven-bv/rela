//go:build sqlite

package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// storeConfigProvider is the capability layerStoreConfig needs: a store that
// can hand out a reader over the operator config it carries.
//
// Declared at the call site, but in terms of the store's own ConfigReader
// rather than config.Loader. Go matches method sets exactly, so the assertion
// is on a RETURN type — and arch-lint forbids a store importing internal/config
// to name it. sqlitestore therefore declares an identical two-method interface
// of its own, and the two are pinned equal by a compile-time assertion in this
// package.
type storeConfigProvider interface {
	ProjectFiles() sqlitestore.ConfigReader
}

// layerStoreConfig puts the project's FILES in front of the config baked into
// its database, so a project with both behaves exactly as it did before the
// database learned to carry any.
//
// That ordering is the design, not an implementation detail. A project holding
// both is a project being edited, and the file the operator just wrote must
// win over the copy baked in at package time; it is also what makes each
// consumer safe to convert one at a time, since a converted call site keeps
// reading the same bytes until the file is actually removed (FEAT-UP14BT).
//
// A store carrying no config layers to nothing, which is the ordinary case for
// every project that has never been packaged.
// The two interfaces must stay identical, or the layering silently stops
// happening: the type assertion below would simply fail, with no error
// anywhere. This assertion is what turns that into a compile failure.
var _ config.Loader = (sqlitestore.ConfigReader)(nil)

func layerStoreConfig(disk config.Loader, st store.Store) config.Loader {
	provider, ok := st.(storeConfigProvider)
	if !ok {
		return disk
	}
	layered, err := config.NewLayered(disk, provider.ProjectFiles())
	if err != nil {
		// Only a nil collaborator can fail here and neither is nil on this
		// path. Degrading to files keeps a project with no baked config —
		// every project today — working, rather than refusing to start over a
		// layer it does not use.
		slog.Warn("appbuild: could not layer store-backed config; using files only", "error", err)
		return disk
	}
	return layered
}
