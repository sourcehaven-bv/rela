package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// entityRecreator is the create-only write `rela restore` needs for a deleted
// face (entitymanager.RecreateEntity).
type entityRecreator interface {
	RecreateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
}

// RestoreCmd restores an entity's content and properties to a past version,
// applying the historical snapshot as a normal write through the entitymanager
// (so it is authorized, validated, audited, and itself versioned — no history
// rewriting). If the entity currently exists it is updated; if it was deleted,
// it is re-created.
//
// Scope: entity content + properties only. The entity's relation set as-of the
// version is NOT restored (relation history is a separate capability), and
// file values keep their live value (BUG-CTUW2N): an old value may name bytes
// that are gone. Restore needs a versioning backend (PostgreSQL or SQLite).
//
// Note: the CLI is a full-trust operator surface — per-field write ACL is
// enforced at the data-entry HTTP boundary, not here (consistent with every
// other CLI write, which goes directly through the entitymanager).
type RestoreCmd struct {
	ID      string `arg:"" help:"Entity address to restore: ID, or ID@face for a type with faces."`
	Version int    `arg:"" help:"The 1-based version ordinal to restore to (see 'rela history <address>')."`
}

// Run dispatches `rela restore <address> <version>`.
//
// The address names one face lineage (see historyAddress), and the restore
// writes that face only: an update of the live face, or, for a deleted face,
// a re-create at the same id and face.
func (c *RestoreCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.Versions == nil {
		out.WriteMessage("The active storage backend does not support version history " +
			"(restore needs the PostgreSQL or SQLite build).")
		return nil
	}
	var reader store.HistoryReader = svc.Versions
	ref, err := historyAddress(ctx, svc.Store, svc.Meta, reader, c.ID)
	if err != nil {
		return err
	}

	snap, err := reader.GetVersion(ctx, ref, c.Version)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no version %d for %q", c.Version, ref)
	}
	if err != nil {
		return fmt.Errorf("read version %d for %q: %w", c.Version, ref, err)
	}
	if snap.Face != ref.Face {
		return fmt.Errorf("version %d for %q belongs to face %q", c.Version, ref, snap.Face)
	}

	// Update if the face currently exists, else re-create it. Between this
	// read and the write another writer could delete/recreate the face,
	// flipping the correct branch (TOCTOU); the manager then returns
	// ErrNotFound (update raced a delete) or ErrEntityAlreadyExists (create
	// raced a recreate). Map either to a clear "state changed, retry" message
	// rather than a baffling raw error.
	live, getErr := svc.Store.GetEntity(ctx, ref)
	if getErr != nil {
		live = nil
	}

	// Build the entity to write from the snapshot, keeping the live file
	// values (none on a re-create).
	target := entity.New(ref.ID, snap.Type)
	target.Face = ref.Face
	target.Content = snap.Content
	target.Properties = entitymanager.CarryFileValues(svc.Meta, snap.Type, snap.Properties, live)

	switch {
	case getErr == nil:
		if _, err := svc.EntityManager.UpdateEntity(ctx, target); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("restore %q: entity state changed during restore (deleted concurrently) — re-run", ref)
			}
			return fmt.Errorf("restore (update) %q to v%d: %w", ref, c.Version, err)
		}
	case errors.Is(getErr, store.ErrNotFound):
		// RecreateEntity, not CreateEntity: the face comes back at its own id,
		// which CreateEntity would replace with a new one for any type without
		// manual ids. It authorizes a create on `type@face`, runs no
		// automation (a restore brings back content rather than making a new
		// record), and is create-only: a face recreated since the read above
		// is refused rather than overwritten.
		if _, err := svc.Recreator.RecreateEntity(ctx, target); err != nil {
			if errors.Is(err, entitymanager.ErrEntityAlreadyExists) {
				return fmt.Errorf("restore %q: entity state changed during restore (re-created concurrently) — re-run", ref)
			}
			return fmt.Errorf("restore (re-create) %q to v%d: %w", ref, c.Version, err)
		}
	default: // coverage-ignore: defensive: memstore.GetEntity returns only nil or store.ErrNotFound, so a
		// non-NotFound error here is unreachable
		return fmt.Errorf("restore %q: check current state: %w", ref, getErr)
	}

	out.WriteSuccess("Restored %s to version %d.", ref, c.Version)
	return nil
}
