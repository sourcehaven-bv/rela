//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// dbFileName is the SQLite database inside the project's cache directory.
//
// It lives under .rela/ because that is already where node-local runtime state
// belongs, and because a project directory should still look like a rela
// project: `schema.yaml`, `templates/` and the rest stay on disk, and disk
// stays FIRST in the resolution order.
//
// The database can also CARRY that config (the project_files table), which is
// what makes a single shippable file possible — but as a fallback, not a
// replacement. A project with both is a project being edited, and the file the
// operator just wrote must win over the copy baked in (FEAT-UP14BT).
const dbFileName = "rela.db"

// DatabasePath returns where the project's database lives.
func DatabasePath(paths *project.Context) string {
	return filepath.Join(paths.CacheDir, dbFileName)
}

// New builds the services bundle for the sqlite build: a single-process
// SQLite store plus an in-memory/on-disk bleve index wired as a write
// observer.
//
// This is the per-scenario recipe — it owns only the backend choice; [prepare]
// and [assemble] do the build-agnostic work every build shares.
//
// The database is opened BEFORE [prepare], unlike the other recipes, because
// the database may carry the project's config, schema.yaml and acl.yaml
// included (FEAT-UP14BT). The one handle then serves the config loader, the
// store and the runtime-state overrides.
//
// Pairing SQLite with bleve rather than FTS5 is deliberate for now:
// search.Visible wraps ANY Searcher, so a native FTS5 searcher is a later
// optimization rather than a prerequisite (DEC-LFSYNY stage 3).
func New(cfg Config, opts ...Option) (*Services, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	ctx := context.Background()
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return nil, err
	}

	cfg.projectConfig, err = layerProjectConfig(cfg.Paths.Root, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	base, err := prepare(cfg, opts)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	st, searcher, closer, err := openBackend(ctx, base, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// The runtime state the database holds. Built here rather than in
	// assemble because the handle is this recipe's, not the store's.
	overrides, err := backendServices(db)
	if err != nil {
		_ = closer.Close()
		return nil, err
	}

	// nil VisibleSearcher → assemble derives the generic search.NewVisible
	// wrapper. Only the postgres recipe has a native implementation.
	return assemble(base, st, searcher, nil, closer, overrides)
}

// openDatabase opens the project's SQLite database.
//
// The DATABASE is opened here and owned by this recipe — not by the store. A
// rela database file holds two unrelated things, the entity graph and the
// operator's config, so neither owns the other or the file they share.
func openDatabase(ctx context.Context, cfg Config) (*sqlitedb.DB, error) {
	if cfg.Paths.CacheDir == "" {
		return nil, errors.New("appbuild: sqlite backend requires a project cache directory")
	}
	// A project found by its schema.yaml need not have a .rela/ yet, and the
	// database (with its lock file) is the first thing that lives there.
	if err := os.MkdirAll(cfg.Paths.CacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("appbuild: create cache directory: %w", err)
	}
	// Open's errors are surfaced unchanged: they are the actionable ones —
	// another process holds the single-writer lock, or WAL could not be
	// enabled because the project sits on a network/sync filesystem.
	// Wrapping them in "open store" would bury the part the operator needs.
	return sqlitedb.Open(ctx, sqlitedb.Options{
		Path: DatabasePath(cfg.Paths),
	})
}

// openBackend builds the SQLite store and the bleve-backed searcher over an
// opened database. On success the returned closer owns db; on error the
// caller still does.
//
// Mirrors the filesystem recipe: the index is created first and installed as a
// store observer at open time so it receives write events from the start, then
// backfilled with whatever the database already holds (the observer is not
// invoked for pre-existing rows).
//
// A nil index is non-fatal — the store still opens and the read/write paths
// keep working with an error-Searcher, because losing search is much less bad
// than refusing to start.
func openBackend(
	ctx context.Context, base *SharedBase, db *sqlitedb.DB,
) (store.Store, search.Searcher, io.Closer, error) {
	idx := openSearchIndex(base)

	opts := []sqlitestore.Option{}
	if idx != nil {
		opts = append(opts, sqlitestore.WithObserver(idx))
	}

	st, err := sqlitestore.New(db, opts...)
	if err != nil {
		if idx != nil {
			_ = idx.Close()
		}
		return nil, nil, nil, err
	}

	if idx == nil {
		return st, search.ErrSearcher(errors.New("search index not available")), dbCloser{db: db}, nil
	}
	if err := backfillBleve(ctx, idx, st); err != nil {
		slog.Warn("appbuild: failed to index entities", "error", err)
	}
	return st, search.New(st, idx), bothCloser{db: db, idx: idx}, nil
}

// dbCloser releases the database when there is no search index to close too.
//
// The database needs a closer at all because the store only BORROWS it — the
// file is this recipe's to own, so tearing it down is this recipe's job.
type dbCloser struct{ db *sqlitedb.DB }

func (c dbCloser) Close() error { return c.db.Close() }

// bothCloser releases the search index and then the database.
//
// Index first: it holds no handle on the database, but closing the database
// out from under a still-running index would be the harder failure to
// diagnose of the two.
type bothCloser struct {
	db  *sqlitedb.DB
	idx io.Closer
}

func (c bothCloser) Close() error {
	err := c.idx.Close()
	if dbErr := c.db.Close(); dbErr != nil && err == nil {
		err = dbErr
	}
	return err
}
