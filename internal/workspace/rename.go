package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/rename"
)

// rename performs an entity ID rename. It is a thin adapter over
// [rename.Rename]; the orchestration lives in internal/rename so this
// transitional shim and [internal/entitymanager.Manager] share one
// implementation.
//
// The entityType parameter is informational and is checked against
// the loaded entity's type before any writes.
func (w *Workspace) rename(entityType, oldID, newID string, opts rename.Options) (*rename.Result, error) {
	if opts.EntityType == "" {
		opts.EntityType = entityType
	}
	return rename.Rename(context.Background(), w.Store(), oldID, newID, opts)
}
