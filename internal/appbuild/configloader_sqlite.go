//go:build sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/config/configsql"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

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
