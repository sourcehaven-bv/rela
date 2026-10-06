//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

// DatabasePath returns where the project's database lives: the document
// file itself when the project was opened as one, else .rela/rela.db.
func DatabasePath(paths *project.Context) string {
	if paths.DatabaseFile != "" {
		return paths.DatabaseFile
	}
	return filepath.Join(paths.CacheDir, dbFileName)
}

// New builds the services bundle for the sqlite build: a single-process
// SQLite store searched through FTS5 in the same database.
//
// This is the per-scenario recipe — it owns only the backend choice; [prepare]
// and [assemble] do the build-agnostic work every build shares.
//
// The database is opened BEFORE [prepare], unlike the other recipes, because
// the database may carry the project's config, schema.yaml and acl.yaml
// included (FEAT-UP14BT). The one handle then serves the config loader, the
// store and the runtime-state overrides.
func New(cfg Config, opts ...Option) (*Services, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if KeepsMarkdownData(cfg.Paths) {
		// Opening a database here would show an empty project and keep
		// every edit out of the files the operator versions.
		return newFS(cfg, opts...)
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

	st, searcher, closer, err := openBackend(base, db)
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

// KeepsMarkdownData reports whether the project keeps its data in markdown
// files rather than in a database: it has no database yet, and its entities/
// or relations/ directory holds something. Such a project opens on the
// filesystem store, as it would in the default build, until its data is
// imported (`rela db load --data`, or the desktop's import), which creates
// the database and from then on is what opens.
//
// "Holds something" means a file at any depth: a new project's empty
// per-type directories are not data, and it opens on the database.
func KeepsMarkdownData(paths *project.Context) bool {
	if _, err := os.Lstat(DatabasePath(paths)); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	return hasFile(paths.EntitiesDir) || hasFile(paths.RelationsDir)
}

// hasFile reports whether dir holds a non-directory entry at any depth.
func hasFile(dir string) bool {
	found := errors.New("found")
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return found
		}
		return nil
	})
	return errors.Is(err, found)
}

// openExistingDatabase opens the project's database, refusing with
// [ErrNoDatabase] when there is none. Operations that only read it use this:
// creating a database as a side effect would switch a markdown project over
// to an empty one (see [KeepsMarkdownData]).
func openExistingDatabase(ctx context.Context, paths *project.Context) (*sqlitedb.DB, error) {
	if _, err := os.Lstat(DatabasePath(paths)); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", DatabasePath(paths), ErrNoDatabase)
	}
	return openDatabase(ctx, Config{Paths: paths})
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

// openBackend builds the SQLite store and its FTS5 searcher (DEC-10Z731)
// over an opened database. The search index lives in the database and the
// database's triggers keep it current, so there is no index to open, backfill
// or close. On success the returned closer owns db; on error the caller still
// does.
func openBackend(base *SharedBase, db *sqlitedb.DB) (store.Store, search.Searcher, io.Closer, error) {
	backend, err := sqlitestore.NewSearchBackend(db)
	if err != nil {
		return nil, nil, nil, err
	}
	backend.RankByTitles(sqlitestore.SearchTitles(rankingTitles(base.meta)))
	st, err := sqlitestore.New(db)
	if err != nil {
		return nil, nil, nil, err
	}
	return st, search.New(st, backend), dbCloser{db: db}, nil
}

// dbCloser releases the database.
//
// The database needs a closer at all because the store only BORROWS it — the
// file is this recipe's to own, so tearing it down is this recipe's job.
type dbCloser struct{ db *sqlitedb.DB }

func (c dbCloser) Close() error { return c.db.Close() }
