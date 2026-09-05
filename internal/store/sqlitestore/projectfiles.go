package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"
)

// ProjectFiles is the operator-authored config carried IN the database:
// schema.yaml, data-entry.yaml, acl.yaml, scripts/, templates/, custom/. It is
// what lets a single file be a complete, shippable rela project rather than
// the data half of one.
//
// It structurally satisfies the config.Loader interface (Load + List) without
// importing it: arch-lint forbids a store depending on an application package,
// so the wiring site declares the narrow interface it needs instead. Same
// arrangement as pgstore.StateKV and state.KV.
//
// Nil: never returned nil by [Conn.ProjectFiles].
type ProjectFiles struct {
	db *sql.DB
}

// ProjectFiles returns a reader over the config stored in this database.
//
// Available before a store exists, which is the point of the connection split:
// the metamodel has to be loaded before anything that consumes one, the store
// included.
func (c *Conn) ProjectFiles() *ProjectFiles {
	return &ProjectFiles{db: c.db}
}

// Load returns the bytes stored at name.
//
// Errors: an absent row returns an [fs.ErrNotExist]-compatible error. That is
// the contract every config consumer branches on — a layered loader falls
// through to the next source on exactly this error and on nothing else, so a
// different error here would make a baked-in file shadow the one on disk.
func (p *ProjectFiles) Load(ctx context.Context, name string) ([]byte, error) {
	if err := validateProjectPath(name); err != nil {
		return nil, err
	}
	var content []byte
	err := p.db.QueryRowContext(ctx,
		`SELECT content FROM project_files WHERE path = ?`, name,
	).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: read project file %q: %w", name, err)
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
func (p *ProjectFiles) List(ctx context.Context, dir string) ([]string, error) {
	if err := validateProjectPath(dir); err != nil {
		return nil, err
	}
	prefix := dir + "/"
	rows, err := p.db.QueryContext(ctx,
		`SELECT path FROM project_files WHERE substr(path, 1, ?) = ?`,
		len(prefix), prefix)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list project files under %q: %w", dir, err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var path string
		if scanErr := rows.Scan(&path); scanErr != nil {
			return nil, fmt.Errorf("sqlitestore: list project files under %q: %w", dir, scanErr)
		}
		rest := path[len(prefix):]
		if strings.Contains(rest, "/") {
			continue // a deeper path, not an entry of this directory
		}
		names = append(names, rest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: list project files under %q: %w", dir, err)
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
// edited row by row — that is what keeps the on-disk files the thing an
// operator actually edits.
func (p *ProjectFiles) Put(ctx context.Context, name string, content []byte) error {
	if err := validateProjectPath(name); err != nil {
		return err
	}
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO project_files (path, content, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET content = excluded.content,
		                                 updated_at = excluded.updated_at`,
		name, content, time.Now().UTC().Format(timeFmt))
	if err != nil {
		return fmt.Errorf("sqlitestore: write project file %q: %w", name, err)
	}
	return nil
}

// Paths returns every stored path, sorted. It backs `rela db dump`.
func (p *ProjectFiles) Paths(ctx context.Context) ([]string, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT path FROM project_files`)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list project files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var path string
		if scanErr := rows.Scan(&path); scanErr != nil {
			return nil, fmt.Errorf("sqlitestore: list project files: %w", scanErr)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: list project files: %w", err)
	}
	slices.Sort(paths)
	return paths, nil
}

// validateProjectPath applies the same rules the filesystem loader applies to
// a name, so the two backends accept and reject exactly the same set.
//
// A database key needs no traversal defence of its own — there is no ".." to
// resolve in a column — but the backends must AGREE, or a path that works on
// disk would fail once baked in: at load time, on a project that was fine the
// day before.
func validateProjectPath(name string) error {
	if name == "" {
		return errors.New("sqlitestore: project file name must not be empty")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.New("sqlitestore: control character (including NUL) not allowed")
		}
	}
	if strings.ContainsRune(name, '\\') {
		return errors.New("sqlitestore: backslash not allowed (use forward slash)")
	}
	if strings.HasPrefix(name, "/") {
		return errors.New("sqlitestore: project file name must be relative")
	}
	for seg := range strings.SplitSeq(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("sqlitestore: traversal or empty segment not allowed")
		}
	}
	if len(name) >= 2 && name[1] == ':' {
		return errors.New("sqlitestore: drive letter not allowed")
	}
	return nil
}
