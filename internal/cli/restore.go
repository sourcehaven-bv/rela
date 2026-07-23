package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RestoreCmd restores an entity's content and properties to a past version,
// applying the historical snapshot as a normal write through the entitymanager
// (so it is authorized, validated, audited, and itself versioned — no history
// rewriting). If the entity currently exists it is updated; if it was deleted,
// it is re-created.
//
// Scope: entity content + properties only. The entity's relation set as-of the
// version is NOT restored (relation history is a separate capability). Restore
// is a pgstore-only capability.
//
// Note: the CLI is a full-trust operator surface — per-field write ACL is
// enforced at the data-entry HTTP boundary, not here (consistent with every
// other CLI write, which goes directly through the entitymanager).
type RestoreCmd struct {
	ID      string `arg:"" help:"Entity ID to restore."`
	Version int    `arg:"" help:"The 1-based version ordinal to restore to (see 'rela history <id>')."`
}

// Run dispatches `rela restore <id> <version>`.
func (c *RestoreCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.Versions == nil {
		out.WriteMessage("The active storage backend does not support version history " +
			"(restore is a PostgreSQL-build feature).")
		return nil
	}
	var reader store.HistoryReader = svc.Versions

	snap, err := reader.GetVersion(ctx, c.ID, c.Version)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no version %d for %q", c.Version, c.ID)
	}
	if err != nil {
		return fmt.Errorf("read version %d for %q: %w", c.Version, c.ID, err)
	}

	// Build the entity to write from the snapshot.
	target := entity.New(c.ID, snap.Type)
	target.Content = snap.Content
	target.Properties = snap.Properties

	// Update if the entity currently exists, else re-create it. Between this
	// read and the write another writer could delete/recreate the entity,
	// flipping the correct branch (TOCTOU); the manager then returns
	// ErrNotFound (update raced a delete) or ErrEntityAlreadyExists (create
	// raced a recreate). Map either to a clear "state changed, retry" message
	// rather than a baffling raw error.
	_, getErr := svc.Store.GetEntity(ctx, c.ID)
	switch {
	case getErr == nil:
		if _, err := svc.EntityManager.UpdateEntity(ctx, target); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("restore %q: entity state changed during restore (deleted concurrently) — re-run", c.ID)
			}
			return fmt.Errorf("restore (update) %q to v%d: %w", c.ID, c.Version, err)
		}
	case errors.Is(getErr, store.ErrNotFound):
		if _, err := svc.EntityManager.CreateEntity(ctx, target, entity.CreateOptions{}); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return fmt.Errorf("restore %q: entity state changed during restore (re-created concurrently) — re-run", c.ID)
			}
			return fmt.Errorf("restore (re-create) %q to v%d: %w", c.ID, c.Version, err)
		}
	default:
		return fmt.Errorf("restore %q: check current state: %w", c.ID, getErr)
	}

	out.WriteSuccess("Restored %s to version %d.", c.ID, c.Version)
	return nil
}
