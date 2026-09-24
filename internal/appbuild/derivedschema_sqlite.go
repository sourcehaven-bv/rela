//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// reconcileDerivedSchemaIfSupported converges the derived query and list
// indexes the SQL graph path uses (TKT-B51CYD). Failures are logged and
// swallowed, as on postgres: a missing index costs speed, never correctness.
//
// `unique:` specs are not derived here. The application-level check-then-write
// scan stays the enforcement (TKT-L1A3PH), which is sound only because there is
// exactly one writer PER PROJECT DATABASE, and sqlitestore makes that true
// rather than assuming it: Open takes an exclusive lock on a sidecar file and
// refuses a second opener. Were that lock ever removed, the scan would become a
// correctness hole with no backstop. That is also why Open verifies WAL
// engaged: the filesystems where flock is unreliable are the ones without WAL.
func reconcileDerivedSchemaIfSupported(
	ctx context.Context, st store.Store, base *SharedBase, cfg config.Loader,
) {
	s, ok := st.(*sqlitestore.Store)
	if !ok {
		return
	}
	specs, err := staticIndexSpecs(ctx, base.meta, cfg)
	if err != nil {
		slog.Warn("appbuild: derived-index reconcile skipped", "error", err)
		return
	}
	outcomes, err := sqlitestore.Reconcile(ctx, s, specs, store.ReconcileOptions{})
	if err != nil {
		slog.Warn("appbuild: derived-index reconcile failed; query indexes may be stale", "error", err)
		return
	}
	logDerivedOutcomes(outcomes)
}

// ErrNoDatabase reports that a dry run found no database to compare with.
var ErrNoDatabase = errors.New("no database yet")

// ReconcileDerivedIndexes opens the project's database, derives the desired
// index set exactly as boot does, and converges it; with opts.DryRun it only
// reports. It backs `rela db reconcile`. Unlike boot, an unreadable or invalid
// data-entry config is an error here: the operator asked for a reconcile, and
// silently doing nothing would read as "up to date".
//
// The database admits one process at a time, so this fails while a server
// holds the project open.
//
// A dry run changes nothing, including the database's existence and schema
// version: opening a database creates and migrates it, so the dry run checks
// both read-only first. With no database it returns [ErrNoDatabase]; with an
// older schema it refuses.
func ReconcileDerivedIndexes(
	ctx context.Context, fs storage.FS, paths *project.Context, opts store.ReconcileOptions,
) ([]store.DerivedObjectOutcome, error) {
	meta, _, err := metamodel.NewFSLoader(fs, paths.SchemaPath).Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	path := filepath.Join(paths.CacheDir, dbFileName)
	if opts.DryRun {
		if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
			return nil, ErrNoDatabase
		}
		found, want, statusErr := sqlitedb.Status(ctx, path)
		if statusErr != nil {
			return nil, statusErr
		}
		if found < want {
			return nil, fmt.Errorf("database is at schema version %d, this binary expects %d; "+
				"run 'rela db migrate' first", found, want)
		}
	}
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	cfg, err := layerProjectConfig(config.NewFSLoader(fs, paths.Root), db)
	if err != nil {
		return nil, err
	}
	specs, err := staticIndexSpecs(ctx, meta, cfg)
	if err != nil {
		return nil, err
	}
	st, err := sqlitestore.New(db)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()
	return sqlitestore.Reconcile(ctx, st, specs, opts)
}
