package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// renameEntity renames an entity via the store's atomic
// [store.Store.RenameEntity] — a single backend operation that re-keys the
// entity and every incident relation together (one transaction on
// pgstore). Routing through the store retires the lost-update / clobber /
// partial-failure class that the old store-agnostic decompose-into-
// create+delete path carried (BUG-5QDV6F): there is no non-atomic window
// for a concurrent writer to slip into.
//
// DryRun is not a store capability, so it is planned here read-only:
// verify the rename would succeed and count the relations that would move,
// without mutating anything.
func renameEntity(
	ctx context.Context, st store.Store, oldID, newID string, opts entity.RenameOptions,
) (*entity.RenameResult, error) {
	if opts.DryRun {
		return planRename(ctx, st, oldID, newID)
	}

	res, err := st.RenameEntity(ctx, oldID, newID)
	if err != nil {
		return nil, translateRenameErr(err, oldID, newID)
	}
	return &entity.RenameResult{
		OldID:            oldID,
		NewID:            newID,
		RelationsUpdated: res.RelationsUpdated,
	}, nil
}

// planRename reports what a rename would do without persisting anything.
// It mirrors the store's precondition order (new ID well-formed, old
// exists, new free) so a dry run and the real write agree on which errors
// fire, and counts incident relations the way the store reports them —
// each once, self-referential edges included.
func planRename(ctx context.Context, st store.Store, oldID, newID string) (*entity.RenameResult, error) {
	if err := entity.ValidateID(newID); err != nil {
		return nil, fmt.Errorf("invalid new ID: %w", err)
	}
	// The whole family, not the zero face: a faced type stores no row there,
	// so a zero-face read reported every faced entity missing.
	family, err := familyRows(ctx, st, oldID)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, oldID)
	}
	taken, err := idTakenByOther(ctx, st, newID, oldID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, fmt.Errorf("%w: %s", ErrEntityAlreadyExists, newID)
	}

	updated := 0
	for _, err := range st.ListRelations(ctx, store.RelationQuery{
		EntityID: oldID, Direction: store.DirectionBoth,
	}) {
		if err != nil {
			continue
		}
		updated++
	}
	return &entity.RenameResult{OldID: oldID, NewID: newID, RelationsUpdated: updated}, nil
}

// idTakenByOther reports whether any stored face has an id equal to id under
// case folding, other than the family except (the entity being renamed, so
// abc -> ABC is allowed). It matches the stores' own conflict rule, so a dry
// run refuses exactly the renames the store refuses.
//
// It scans content-free headers. A dry run is an explicit operator request
// for one entity, not a collection read, and no store query folds ids.
func idTakenByOther(ctx context.Context, st store.Store, id, except string) (bool, error) {
	folded, exceptFolded := strings.ToLower(id), strings.ToLower(except)
	for h, err := range store.ListEntityHeaders(ctx, st, store.EntityQuery{Faces: store.AllFaces()}) {
		if err != nil {
			return false, err
		}
		if f := strings.ToLower(h.ID); f == folded && f != exceptFolded {
			return true, nil
		}
	}
	return false, nil
}

// translateRenameErr maps the store's rename sentinels into
// entitymanager's so callers only have to know one set of error values.
func translateRenameErr(err error, oldID, newID string) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("%w: %s", ErrEntityNotFound, oldID)
	case errors.Is(err, store.ErrConflict):
		return fmt.Errorf("%w: %s", ErrEntityAlreadyExists, newID)
	default:
		return err
	}
}
