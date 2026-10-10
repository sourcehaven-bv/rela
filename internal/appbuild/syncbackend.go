package appbuild

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// RequireSyncBackend refuses a schema that declares `sync: true` external
// refs on a backend without version history (TKT-SM20FG, D11). A sync
// connector merges against the version it tagged at its last sync, so on
// such a backend it would run with no base and could only report
// conflicts.
//
// The long-running hosts call it at startup: rela-server, rela-desktop and
// `rela scheduler`. It is not part of assembly, so read-only tooling (show,
// list, analyze, migrations) still opens such a project on any backend.
// A package function: Services sits at its plimsoll exported-method line.
func RequireSyncBackend(s *Services) error {
	props := metamodel.SyncRefProps(s.meta)
	if len(props) == 0 || versionTagReaderFor(s.store) != nil {
		return nil
	}
	return fmt.Errorf(
		"schema declares sync-managed external refs (%s); sync needs version history, "+
			"which only the SQLite and PostgreSQL builds keep", strings.Join(props, ", "))
}
