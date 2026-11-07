package dataentry

import (
	"log/slog"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// elevatedUpdaterFactory builds the admin.update_entity writer for one action
// run. scope is the action's elevated_updates block; source names the action
// in the audit log. bound is the raw row an entity-bound action runs
// against, or nil; an elevated write to it fails when it changed since.
type elevatedUpdaterFactory func(
	scope map[string][]string, source string, bound *entity.Entity,
) (lua.ElevatedUpdater, error)

// elevatedUpdatesFor returns the factory for actions that declare
// allow_acl_bypass: write (TKT-JA5SABG). The writer it builds may change only
// the types and properties the action lists under elevated_updates.
//
// Nil em: NewApp already rejects it; [entitymanager.NewScopedUpdates] would
// too, so the action fails closed.
func elevatedUpdatesFor(em *entitymanager.Manager) elevatedUpdaterFactory {
	return func(scope map[string][]string, source string, bound *entity.Entity) (lua.ElevatedUpdater, error) {
		u, err := entitymanager.NewScopedUpdates(em, entitymanager.UpdateScope(scope), source)
		if err != nil {
			return nil, err
		}
		u.Pin(bound)
		return u, nil
	}
}

// grantElevatedUpdates sets deps.ElevatedUpdater when the action declares
// allow_acl_bypass: write. Config validation guarantees such an action has a
// permission and a non-empty elevated_updates block. When the writer cannot
// be built, the action fails with 500 rather than run without the writes the
// operator declared. bound goes to the factory; see [elevatedUpdaterFactory].
func grantElevatedUpdates(
	factory elevatedUpdaterFactory, deps *lua.WriteDeps, id string, action dataentryconfig.Action,
	bound *entity.Entity,
) *opFailure {
	if !action.AllowACLBypass.AllowsWrite() {
		return nil
	}
	if factory == nil {
		slog.Error("elevated action has no updater", "action", id)
		return failV1(http.StatusInternalServerError, "elevation_unavailable",
			"elevated updates are not available")
	}
	u, err := factory(action.ElevatedUpdates, "action:"+id, bound)
	if err != nil {
		slog.Error("elevated action updater", "action", id, "error", err)
		return failV1(http.StatusInternalServerError, "elevation_unavailable",
			"elevated updates are not available")
	}
	deps.ElevatedUpdater = u
	return nil
}
