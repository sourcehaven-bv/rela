//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// dbFileName is the SQLite database inside the project's cache directory.
//
// It lives under .rela/ because that is already where node-local runtime state
// belongs, and because a project directory should still look like a rela
// project — `schema.yaml`, `templates/` and the rest stay on disk exactly as
// they do on the postgres build. SQLite backs entities, relations and
// attachments; it does not swallow operator-authored config.
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
	st, searcher, closer, err := openBackend(context.Background(), base)
	if err != nil {
		return nil, err
	}
	// nil VisibleSearcher → assemble derives the generic search.NewVisible
	// wrapper. Only the postgres recipe has a native implementation.
	return assemble(base, st, searcher, nil, closer)
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
func openBackend(ctx context.Context, base *SharedBase) (store.Store, search.Searcher, io.Closer, error) {
	if base.cfg.Paths.CacheDir == "" {
		return nil, nil, nil, errors.New("appbuild: sqlite backend requires a project cache directory")
	}

	idx := openSearchIndex(base)

	opts := []sqlitestore.Option{}
	if idx != nil {
		opts = append(opts, sqlitestore.WithObserver(idx))
	}

	st, err := sqlitestore.OpenContext(ctx, sqlitestore.Options{
		Path: filepath.Join(base.cfg.Paths.CacheDir, dbFileName),
	}, opts...)
	if err != nil {
		// Surfaced unchanged: Open's errors are the actionable ones — another
		// process holds the single-writer lock, or WAL could not be enabled
		// because the project sits on a network/sync filesystem. Wrapping them
		// in "open store" would bury the part the operator needs.
		return nil, nil, nil, err
	}

	if idx == nil {
		return st, search.ErrSearcher(errors.New("search index not available")), noopSQLiteCloser{}, nil
	}
	if err := backfillBleve(ctx, idx, st); err != nil {
		slog.Warn("appbuild: failed to index entities", "error", err)
	}
	return st, search.New(st, idx), idx, nil
}

// noopSQLiteCloser satisfies the io.Closer assemble tears down when there is no
// search index to close. Declared per-recipe like the other builds' equivalents.
type noopSQLiteCloser struct{}

func (noopSQLiteCloser) Close() error { return nil }
