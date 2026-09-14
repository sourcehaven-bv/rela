// Package configsql serves a project's operator-authored config — schema.yaml,
// data-entry.yaml, acl.yaml, scripts/, templates/, custom/ — from a SQL table
// rather than from files on disk. It is what lets a single database file be a
// complete, shippable rela project rather than the data half of one.
//
// It is NOT part of any storage backend. Nothing here touches an entity, a
// relation or a graph; it takes a *sql.DB and reads rows keyed by path. That
// separation is the point: config and data merely share a file, and a store's
// job is the graph. Keeping this out of the store also keeps the backend the
// conformance suite describes — the minimal one — actually minimal.
//
// The handle is supplied by whoever opened the database; sqlitestore.Conn.DB
// hands one out before a store exists, which is the ordering config needs
// (the metamodel loads before anything that consumes a metamodel).
package configsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/config"
)

// timeFmt is the on-disk timestamp format, matching what the store writes to
// its own rows: RFC3339Nano, timezone retained.
const timeFmt = time.RFC3339Nano

// Loader reads config from the project_files table. It implements
// [config.Loader], declared rather than merely structural — this package may
// import config, being a config package rather than a storage one.
//
// Nil: [New] rejects a nil handle; a nil *Loader is a programming error.
type Loader struct {
	db *sql.DB
}

var _ config.Loader = (*Loader)(nil)

// New returns a Loader over db.
//
// The caller owns db and must not hand over a handle it will close first —
// this borrows the connection, it does not take ownership of it.
func New(db *sql.DB) (*Loader, error) {
	if db == nil {
		return nil, errors.New("configsql: a non-nil database handle is required")
	}
	return &Loader{db: db}, nil
}

// Load returns the bytes stored at name.
//
// Errors: an absent row returns an [fs.ErrNotExist]-compatible error. That is
// the contract every config consumer branches on — a layered loader falls
// through to the next source on exactly this error and on nothing else, so a
// different error here would make a baked-in file shadow the one on disk.
func (l *Loader) Load(ctx context.Context, name string) ([]byte, error) {
	if err := validatePath(name); err != nil {
		return nil, err
	}
	var content []byte
	err := l.db.QueryRowContext(ctx,
		`SELECT content FROM project_files WHERE path = ?`, name,
	).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if err != nil {
		return nil, fmt.Errorf("configsql: read project file %q: %w", name, err)
	}
	return content, nil
}

// List returns the sorted names stored directly under dir, without the dir
// prefix and without descending into subdirectories.
//
// An absent directory lists EMPTY with a nil error, matching the filesystem
// loader. The asymmetry is load-bearing on the consuming side: a project
// legitimately has no scripts/, while an unreadable one reported as "nothing
// here" would silently drop operator-authored config — so only absence is
// forgiven and every real error surfaces.
//
// Paths are flat keys, so "directly under" is a literal prefix match with no
// further separator. A GLOB would be the obvious shortcut and is avoided
// deliberately: the caller's dir would become a pattern, so a directory named
// with a `*`, `?` or `[` would silently match the wrong rows.
func (l *Loader) List(ctx context.Context, dir string) ([]string, error) {
	if err := validatePath(dir); err != nil {
		return nil, err
	}
	prefix := dir + "/"
	rows, err := l.db.QueryContext(ctx,
		`SELECT path FROM project_files WHERE substr(path, 1, ?) = ?`,
		len(prefix), prefix)
	if err != nil {
		return nil, fmt.Errorf("configsql: list project files under %q: %w", dir, err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var path string
		if scanErr := rows.Scan(&path); scanErr != nil {
			return nil, fmt.Errorf("configsql: list project files under %q: %w", dir, scanErr)
		}
		rest := path[len(prefix):]
		if strings.Contains(rest, "/") {
			continue // a deeper path, not an entry of this directory
		}
		names = append(names, rest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("configsql: list project files under %q: %w", dir, err)
	}
	// Sorted here rather than by the query: the order is part of this method's
	// contract (a script chain's execution order depends on it), so it must
	// not rest on SQLite's collation happening to agree with Go's.
	slices.Sort(names)
	return names, nil
}

// Put stores content at name, replacing whatever was there.
//
// This is the write half `rela db load` needs. There is deliberately no
// richer editing API: config is loaded as a set and dumped as a set, never
// edited row by row — that is what keeps the files on disk the thing an
// operator actually edits.
//
// Put is on the same type as Load rather than split behind a second
// interface: a caller that only reads config is handed a [config.Loader], and
// that interface has no Put, so the read-only view is already the default one.
func (l *Loader) Put(ctx context.Context, name string, content []byte) error {
	if err := validatePath(name); err != nil {
		return err
	}
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO project_files (path, content, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET content = excluded.content,
		                                 updated_at = excluded.updated_at`,
		name, content, time.Now().UTC().Format(timeFmt))
	if err != nil {
		return fmt.Errorf("configsql: write project file %q: %w", name, err)
	}
	return nil
}

// Paths returns every stored path, sorted. It backs `rela db dump`.
func (l *Loader) Paths(ctx context.Context) ([]string, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT path FROM project_files`)
	if err != nil {
		return nil, fmt.Errorf("configsql: list project files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var path string
		if scanErr := rows.Scan(&path); scanErr != nil {
			return nil, fmt.Errorf("configsql: list project files: %w", scanErr)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("configsql: list project files: %w", err)
	}
	slices.Sort(paths)
	return paths, nil
}

// validatePath applies the same rules the filesystem loader applies to
// a name, so the two backends accept and reject exactly the same set.
//
// A database key needs no traversal defense of its own — there is no ".." to
// resolve in a column — but the backends must AGREE, or a path that works on
// disk would fail once baked in: at load time, on a project that was fine the
// day before.
func validatePath(name string) error {
	if name == "" {
		return errors.New("configsql: project file name must not be empty")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.New("configsql: control character (including NUL) not allowed")
		}
	}
	if strings.ContainsRune(name, '\\') {
		return errors.New("configsql: backslash not allowed (use forward slash)")
	}
	if strings.HasPrefix(name, "/") {
		return errors.New("configsql: project file name must be relative")
	}
	for seg := range strings.SplitSeq(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("configsql: traversal or empty segment not allowed")
		}
	}
	if len(name) >= 2 && name[1] == ':' {
		return errors.New("configsql: drive letter not allowed")
	}
	return nil
}
