package entitymanager

import (
	"context"
	"errors"
	"fmt"

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
	if _, err := st.GetEntity(ctx, oldID); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, oldID)
	}
	if _, err := st.GetEntity(ctx, newID); err == nil {
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
