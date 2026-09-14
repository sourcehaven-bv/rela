//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"

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

// New builds the services bundle for the sqlite build: a single-process
// SQLite store plus an in-memory/on-disk bleve index wired as a write
// observer.
//
// This is the per-scenario recipe — it owns only the backend choice; [prepare]
// and [assemble] do the build-agnostic work every build shares.
//
// Pairing SQLite with bleve rather than FTS5 is deliberate for now:
// search.Visible wraps ANY Searcher, so a native FTS5 searcher is a later
// optimization rather than a prerequisite (DEC-LFSYNY stage 3).
func New(cfg Config, opts ...Option) (*Services, error) {
	base, err := prepare(cfg, opts)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	db, st, searcher, closer, err := openBackend(ctx, base)
	if err != nil {
		return nil, err
	}

	// Both overrides come from the one handle this recipe opened: the config
	// the database carries (layered behind the files) and the runtime state it
	// holds. Built here rather than in assemble because the handle is this
	// recipe's, not the store's.
	overrides, err := backendServices(cfg, db)
	if err != nil {
		_ = closer.Close()
		return nil, err
	}

	// nil VisibleSearcher → assemble derives the generic search.NewVisible
	// wrapper. Only the postgres recipe has a native implementation.
	return assemble(base, st, searcher, nil, closer, overrides)
}

// openBackend opens the SQLite store and the bleve-backed searcher.
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
	ctx context.Context, base *SharedBase,
) (*sqlitedb.DB, store.Store, search.Searcher, io.Closer, error) {
	if base.cfg.Paths.CacheDir == "" {
		return nil, nil, nil, nil, errors.New("appbuild: sqlite backend requires a project cache directory")
	}

	idx := openSearchIndex(base)

	opts := []sqlitestore.Option{}
	if idx != nil {
		opts = append(opts, sqlitestore.WithObserver(idx))
	}

	// The DATABASE is opened here and owned here — not by the store. A rela
	// database file holds two unrelated things, the entity graph and the
	// operator's config, so neither owns the other or the file they share:
	// this recipe opens once and hands the same handle to both.
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{
		Path: filepath.Join(base.cfg.Paths.CacheDir, dbFileName),
	})
	if err != nil {
		// Surfaced unchanged: Open's errors are the actionable ones — another
		// process holds the single-writer lock, or WAL could not be enabled
		// because the project sits on a network/sync filesystem. Wrapping them
		// in "open store" would bury the part the operator needs.
		return nil, nil, nil, nil, err
	}

	st, err := sqlitestore.New(db, opts...)
	if err != nil {
		_ = db.Close()
		return nil, nil, nil, nil, err
	}

	if idx == nil {
		return db, st, search.ErrSearcher(errors.New("search index not available")), dbCloser{db: db}, nil
	}
	if err := backfillBleve(ctx, idx, st); err != nil {
		slog.Warn("appbuild: failed to index entities", "error", err)
	}
	return db, st, search.New(st, idx), bothCloser{db: db, idx: idx}, nil
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

// noopSQLiteCloser is the io.Closer assemble tears down when there is no search
// index to close.
//
// Declared here rather than in bleveindex_shared.go despite that file being
// compiled into this build too: the fs recipe has its own noopCloser, so a
// shared one would be unused on the default build — dead code the linter
// rightly rejects. Sharing it would mean also removing the fs copy, which is
// a change to the filesystem recipe that this ticket has no reason to make.
type noopSQLiteCloser struct{}

func (noopSQLiteCloser) Close() error { return nil }
