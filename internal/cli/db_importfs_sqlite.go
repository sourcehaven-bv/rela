//go:build sqlite

package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/fsimport"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// dbFileName is the database file inside .rela/, the name
// appbuild.DatabasePath opens.
const dbFileName = "rela.db"

// runDBImportFS copies the filesystem project at source into a new SQLite
// project at target.
func runDBImportFS(ctx context.Context, source, target string) error {
	rep, err := fsimport.Run(ctx, fsimport.Options{
		Source:    source,
		Target:    target,
		Backend:   sqliteImportBackend(),
		Principal: principal.From(ctx),
		Progress:  os.Stderr,
	})
	printImportReport(os.Stdout, rep, err)
	return err
}

// sqliteImportBackend writes the import into .rela/rela.db, the file the
// SQLite build opens.
func sqliteImportBackend() fsimport.Backend {
	dbPath := func(root string) string { return filepath.Join(root, ".rela", dbFileName) }
	return fsimport.Backend{
		Open: func(ctx context.Context, root string) (*fsimport.Opened, error) {
			data, err := appbuild.OpenSQLiteData(ctx, dbPath(root))
			if err != nil {
				return nil, err
			}
			return &fsimport.Opened{
				Target: fsimport.Target{
					Store:      data.Store,
					State:      data.State,
					Comments:   data.Comments,
					Migrations: data.Migrations,
				},
				Close: func(ctx context.Context) error {
					return errors.Join(data.Checkpoint(ctx), data.Close())
				},
			}, nil
		},
		Finish: func(ctx context.Context, root string) error {
			fsys := storage.NewSafeFS(storage.NewOsFS())
			paths, err := project.At(root, fsys)
			if err != nil {
				return err
			}
			if _, err := appbuild.ReconcileDerivedIndexes(ctx, fsys, paths, store.ReconcileOptions{}); err != nil {
				return fmt.Errorf("build the derived indexes: %w", err)
			}
			// The directory is renamed next. A write-ahead log left beside
			// the database would hold committed rows the file alone lacks.
			for _, side := range []string{"-wal", "-shm"} {
				_, err := os.Stat(dbPath(root) + side)
				switch {
				case err == nil:
					return fmt.Errorf("the database left %s%s behind after closing", dbFileName, side)
				case !errors.Is(err, fs.ErrNotExist):
					return err
				}
			}
			return nil
		},
		Owned: []string{filepath.Join(".rela", dbFileName)},
	}
}
