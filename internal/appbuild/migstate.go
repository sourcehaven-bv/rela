package appbuild

import (
	"context"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/filemigstate"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/memmigstate"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// buildMigState selects the migration-state backend for one assembled store.
//
// A recipe-supplied store wins; otherwise the project directory gets the
// committed-file backend. Same precedence as the config loader and comments,
// and for the same reason: the recipe knows which storage tier it built, and
// the default belongs to the filesystem tier.
//
// The fallback when there is no project directory is [memmigstate], not a
// no-op: a store whose migration record silently vanished would re-run every
// migration on the next start. In-memory at least keeps one process
// self-consistent, and a tier with no project directory has no migrations to
// run anyway.
func buildMigState(
	fsys storage.FS, paths *project.Context, override datamigration.StateStore, kv state.KV,
) (datamigration.StateStore, error) {
	next, err := selectMigState(fsys, paths, override)
	if err != nil {
		return nil, err
	}
	// Wrapped so an upgrade adopts the pre-TKT-XCJ0Y2 state.KV marker instead
	// of re-baselining over it, and so a rollback finds a current one. The
	// wrapper is transitional; drop it once no supported version reads the
	// legacy key.
	return datamigration.NewLegacyBridge(next, kv)
}

func selectMigState(
	fsys storage.FS, paths *project.Context, override datamigration.StateStore,
) (datamigration.StateStore, error) {
	if override != nil {
		return override, nil
	}
	if paths == nil || paths.Root == "" || fsys == nil {
		return memmigstate.New(), nil
	}
	st, err := filemigstate.New(fsys, paths.Root)
	if err != nil {
		return nil, fmt.Errorf("appbuild: migration state: %w", err)
	}
	return st, nil
}

// hasMigrationsVia reports whether the project holds any migration files,
// reading through the config loader so a project whose config lives in a
// SQLite file answers as well as one on disk.
//
// It feeds the gate's un-baselined check, which only fires when NO migration
// state is recorded — the rare path — so listing the directory there costs
// nothing on a normal start.
func hasMigrationsVia(loader config.Loader) func(context.Context) (bool, error) {
	if loader == nil {
		return nil
	}
	return func(ctx context.Context) (bool, error) {
		names, err := loader.List(ctx, datamigration.MigrationsDir)
		if err != nil {
			return false, err
		}
		return slices.ContainsFunc(names, datamigration.IsMigrationFileName), nil
	}
}
