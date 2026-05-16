package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// renameEntity is a thin adapter over [rename.Rename]. The
// orchestration lives in internal/rename so this and the legacy
// workspace shim share one implementation.
func renameEntity(
	ctx context.Context, st store.Store, oldID, newID string, opts entity.RenameOptions,
) (*entity.RenameResult, error) {
	res, err := rename.Rename(ctx, st, oldID, newID, rename.Options{DryRun: opts.DryRun})
	if err != nil {
		// Translate rename's sentinels into entitymanager's so
		// callers only have to know one set of error values.
		switch {
		case errors.Is(err, rename.ErrEntityNotFound):
			return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, oldID)
		case errors.Is(err, rename.ErrEntityAlreadyExists):
			return nil, fmt.Errorf("%w: %s", ErrEntityAlreadyExists, newID)
		default:
			return nil, err
		}
	}
	return &entity.RenameResult{
		OldID:            res.OldID,
		NewID:            res.NewID,
		RelationsUpdated: len(res.RelationsUpdated),
	}, nil
}
