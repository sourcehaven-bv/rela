//go:build sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/sqlitecomments"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/config/configsql"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/sqlitemigstate"
	"github.com/Sourcehaven-BV/rela/internal/rootfs"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/state/statesql"
)

// backendServices builds the overrides that come from the opened database:
// the project's runtime state, its migration record and its comments. The
// project's config comes from the same handle but is set on [Config] before
// [prepare], because the schema itself may live there (see [New]).
//
// All in one place because they share a handle and a rationale — each is
// something that would otherwise live in a file BESIDE the database, and so be
// left behind when the single file is shipped.
func backendServices(db *sqlitedb.DB) (backendOverrides, error) {
	data, err := openDBServices(db)
	if err != nil {
		return backendOverrides{}, err
	}
	return backendOverrides{
		stateKV:      data.kv,
		migState:     data.migState,
		commentStore: data.comments,
	}, nil
}

// dbServices are the stores kept in a rela database file beside the entity
// graph. One struct so that [backendServices] and [OpenSQLiteData] cannot
// disagree about what the file holds.
type dbServices struct {
	kv       state.KV
	migState datamigration.StateStore
	comments comments.Store
}

func openDBServices(db *sqlitedb.DB) (dbServices, error) {
	raw, err := statesql.New(db.DB())
	if err != nil {
		return dbServices{}, err
	}
	// The backend stores whatever key it is handed; ValidatedKV applies the
	// key rules FSKV gets from RootedFS, so both backends accept exactly the
	// same keys and neither can drift.
	kv, err := state.NewValidatedKV(raw)
	if err != nil {
		return dbServices{}, err
	}

	// The migration record goes in the database for versioning's reason
	// (TKT-4NU9ZD), not state_kv's: it describes the CONTENT, so a record
	// beside the file would be left behind when rela.db is shipped and the
	// receiving copy would replay every migration (TKT-XCJ0Y2).
	migState, err := sqlitemigstate.New(db.DB())
	if err != nil {
		return dbServices{}, err
	}

	// Comments go in the database rather than .rela/comments/ (TKT-OGTVJW), on
	// the same handle. The reasoning is versioning's (TKT-4NU9ZD), not
	// state.KV's: a comment is content ABOUT content, so it must travel with
	// the rows it annotates — an operator shipping rela.db would otherwise hand
	// over every entity and leave every remark behind.
	commentStore, err := sqlitecomments.New(db.DB())
	if err != nil {
		return dbServices{}, err
	}
	return dbServices{kv: kv, migState: migState, comments: commentStore}, nil
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
//
// The disk layer is a [rootfs.Dir], not a config.FSLoader, because the
// result also serves scripts/, actions/, custom/ and apps/: those readers
// open files through an os.Root per area, which refuses a symlink that
// leaves its directory, and the layered view must keep that.
func layerProjectConfig(root string, db *sqlitedb.DB) (config.Loader, error) {
	baked, err := configsql.New(db.DB())
	if err != nil {
		return nil, err
	}
	return config.NewLayered(rootfs.New(root), baked)
}
