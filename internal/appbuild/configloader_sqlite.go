//go:build sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/config/configsql"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/state/statesql"
)

// backendServices builds the overrides that come from the opened database:
// the project's config and its runtime state.
//
// Both in one place because they share a handle and a rationale — each is
// something that would otherwise live in a file BESIDE the database, and so be
// left behind when the single file is shipped.
func backendServices(cfg Config, db *sqlitedb.DB) (backendOverrides, error) {
	cfgLoader, err := layerProjectConfig(config.NewFSLoader(cfg.FS, cfg.Paths.Root), db)
	if err != nil {
		return backendOverrides{}, err
	}

	raw, err := statesql.New(db.DB())
	if err != nil {
		return backendOverrides{}, err
	}
	// The backend stores whatever key it is handed; ValidatedKV applies the
	// key rules FSKV gets from RootedFS, so both backends accept exactly the
	// same keys and neither can drift.
	kv, err := state.NewValidatedKV(raw)
	if err != nil {
		return backendOverrides{}, err
	}

	return backendOverrides{projectConfig: cfgLoader, stateKV: kv}, nil
}

// layerProjectConfig puts the project's FILES in front of the config baked
// into its database, so a project with both behaves exactly as it did before
// the database learned to carry any.
//
// That ordering is the design, not an implementation detail. A project holding
// both is a project being edited, and the file the operator just wrote must
// win over the copy baked in at package time; it is also what makes each
// consumer safe to convert one at a time, since a converted call site keeps
// reading the same bytes until the file is actually removed (FEAT-UP14BT).
//
// A database carrying no config layers to nothing, which is the ordinary case
// for every project that has never been packaged.
func layerProjectConfig(disk config.Loader, db *sqlitedb.DB) (config.Loader, error) {
	baked, err := configsql.New(db.DB())
	if err != nil {
		return nil, err
	}
	return config.NewLayered(disk, baked)
}
