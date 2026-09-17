// Package sqlitecomments is the SQLite-backed [comments.Store].
//
// The sqlite tier is single-process by construction (sqlitestore takes an
// exclusive sidecar lock, DEC-LFSYNY), so this backend is NOT here to fix a
// multi-writer bug the way pgcomments is. It exists so commentary travels with
// the rows it annotates.
//
// That is the same call content versioning made for this tier (TKT-4NU9ZD) and
// the opposite of the one state.KV made (TKT-L1A3PH), and the difference is the
// point: node-local state buys nothing a single process can observe, so the
// render cache stays in .rela/. But a comment is content ABOUT content — an
// operator who copies or ships rela.db expecting "the project" would find the
// entities present and every remark on them silently left behind in a directory
// they did not know to bring.
//
// # Relationship to the store
//
// This package does not depend on internal/store (arch-lint enforces it); see
// the pgcomments package doc for why commentary stays outside the graph. It
// shares the *sql.DB that sqlitedb already exposes for precisely this purpose
// ("config stored in this database can be read before a store exists and the
// two can share one connection" — sqlitedb.DB.DB), so there is one connection
// pool and one single-writer lock over the file, not two.
package sqlitecomments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// timeFmt is the on-disk timestamp format.
//
// Deliberately NOT sqlitestore.timeFmt (RFC3339Nano), despite the two tables
// living in one file. List orders by this column, and the ordering happens in
// SQL as a STRING compare — which RFC3339Nano gets WRONG, because Go strips
// trailing zeros from the fractional part and omits it entirely at a whole
// second. So "12:00:00Z" sorts AFTER "12:00:00.000000005Z" ('Z' is 0x5A, '.'
// is 0x2E), and a thread would silently reorder between reads.
//
// This format is fixed-width: always nine fractional digits, zero-padded, so
// lexical and chronological order coincide. The store's own columns are not
// changed here — that is a separate concern (its sweep does the same string
// comparison against time windows, where the same flaw is a missed capture
// rather than a visible reordering).
const timeFmt = "2006-01-02T15:04:05.000000000Z07:00"

// DBTX is the database handle this store runs on: the *sql.DB owned by
// sqlitedb.DB in production.
//
// An interface rather than *sql.DB so a caller may supply a transaction or a
// wrapper, and so this package names no concrete database type it does not own.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store is the SQLite comment backend.
type Store struct {
	db DBTX
}

var _ comments.Store = (*Store)(nil)

// New constructs a Store over the given handle.
//
// Nil: rejected, so a wiring mistake fails at construction rather than at the
// first comment anyone posts (CLAUDE.md: constructors reject nil required
// collaborators).
func New(db DBTX) (*Store, error) {
	if db == nil {
		return nil, errors.New("sqlitecomments: New requires a database handle")
	}
	return &Store{db: db}, nil
}

// columns is the read projection, ordered to match scanComment.
const columns = `id, author, created_at, updated_at, anchor, body, resolved`

// List returns the target's thread in contract order: oldest first, the
// server-minted id breaking ties so a coarse clock cannot make a thread
// reorder between reads.
func (s *Store) List(ctx context.Context, target comments.Target) ([]comments.Comment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+columns+`
		FROM comments
		WHERE target_key = ?
		ORDER BY created_at, id`, target.Key())
	if err != nil {
		return nil, fmt.Errorf("sqlitecomments: list %q: %w", target.Key(), err)
	}
	defer rows.Close()

	// Non-nil even when empty: callers marshal straight to JSON, where a nil
	// slice becomes `null` and an empty one `[]`.
	out := []comments.Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlitecomments: list %q: %w", target.Key(), err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitecomments: list %q: %w", target.Key(), err)
	}
	return out, nil
}

// Add inserts one comment, persisting the service-supplied ID, Author and
// CreatedAt as given.
func (s *Store) Add(ctx context.Context, target comments.Target, c comments.Comment) error {
	anchor, err := json.Marshal(c.Anchor)
	if err != nil {
		return fmt.Errorf("sqlitecomments: encode anchor: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO comments
			(id, target_key, target_type, author, created_at, updated_at, anchor, body, resolved)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, target.Key(), target.Type, c.Author,
		c.CreatedAt.UTC().Format(timeFmt), formatOptionalTime(c.UpdatedAt),
		string(anchor), c.Body, c.Resolved)
	if err != nil {
		return fmt.Errorf("sqlitecomments: add to %q: %w", target.Key(), err)
	}
	return nil
}

// Update replaces the mutable fields of one comment.
//
// Author, created_at and anchor are absent from the SET list on purpose: an
// edit must not rewrite who said something or what it was about.
func (s *Store) Update(ctx context.Context, target comments.Target, id, body string, resolved bool) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE comments
		SET body = ?, resolved = ?, updated_at = ?
		WHERE target_key = ? AND id = ?`,
		body, resolved, time.Now().UTC().Format(timeFmt), target.Key(), id)
	if err != nil {
		return fmt.Errorf("sqlitecomments: update %q on %q: %w", id, target.Key(), err)
	}
	return requireAffected(res, comments.ErrNotFound)
}

// Delete removes one comment, reporting [comments.ErrNotFound] if absent.
func (s *Store) Delete(ctx context.Context, target comments.Target, id string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM comments WHERE target_key = ? AND id = ?`, target.Key(), id)
	if err != nil {
		return fmt.Errorf("sqlitecomments: delete %q from %q: %w", id, target.Key(), err)
	}
	return requireAffected(res, comments.ErrNotFound)
}

// DeleteTarget removes one face's thread. An empty target is not an error.
func (s *Store) DeleteTarget(ctx context.Context, target comments.Target) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM comments WHERE target_key = ?`, target.Key()); err != nil {
		return fmt.Errorf("sqlitecomments: delete target %q: %w", target.Key(), err)
	}
	return nil
}

// DeleteAllFaces removes every thread belonging to an entity id — the bare id
// and every "id@face" — so an entity delete strands nothing.
func (s *Store) DeleteAllFaces(ctx context.Context, entityID string) error {
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM comments
		WHERE target_key = ? OR target_key LIKE ? ESCAPE '\'`,
		entityID, facePrefixPattern(entityID)); err != nil {
		return fmt.Errorf("sqlitecomments: delete all faces of %q: %w", entityID, err)
	}
	return nil
}

// Rename re-keys every one of an entity's threads.
//
// Moves EVERY face (re-keying only the bare id would strand a draft thread at
// an id that no longer exists) and MERGES into an occupied destination, since
// rela permits id reuse and discarding the occupant's comments would destroy
// data nobody asked to remove. Both pinned by commentstest.RunRenameTests.
func (s *Store) Rename(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	// substr() is 1-based, so len+1 is the first byte after the old id — the
	// "@face" suffix, or nothing for a bare-id thread. Computed in SQL rather
	// than read back into Go: this runs on the entity write path, and a
	// read-then-write would race a concurrent add to the same thread.
	if _, err := s.db.ExecContext(ctx, `
		UPDATE comments
		SET target_key = ? || substr(target_key, ?)
		WHERE target_key = ? OR target_key LIKE ? ESCAPE '\'`,
		newID, len(oldID)+1, oldID, facePrefixPattern(oldID)); err != nil {
		return fmt.Errorf("sqlitecomments: rename %q to %q: %w", oldID, newID, err)
	}
	return nil
}

// facePrefixPattern builds the LIKE pattern matching every FACED thread of id
// ("id@draft", never the bare "id").
//
// The id is escaped because an entity id may legally contain an underscore
// (entity.ValidateID admits [A-Za-z0-9_-]) and "_" is LIKE's single-character
// wildcard — so renaming "TKT_1" would also re-key "TKT-1", silently merging
// two unrelated threads.
func facePrefixPattern(id string) string {
	r := strings.NewReplacer(`\`, `\\`, `_`, `\_`, `%`, `\%`)
	return r.Replace(id) + entity.StateRefSeparator + "%"
}

// formatOptionalTime maps Go's zero time to SQL NULL, so "never edited" needs
// no sentinel instant. Storing the zero time would round-trip as year 1, which
// reads as a real edit.
func formatOptionalTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(timeFmt)
}

// requireAffected turns a zero-row write into notFound.
//
// SQLite reports RowsAffected reliably for these statements; an error from it
// is a driver failure, not a missing row, so it is returned as-is rather than
// folded into notFound.
func requireAffected(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlitecomments: rows affected: %w", err)
	}
	if n == 0 {
		return notFound
	}
	return nil
}

// scanComment reads one row in [columns] order.
func scanComment(rows *sql.Rows) (comments.Comment, error) {
	var (
		c        comments.Comment
		created  string
		updated  *string
		anchor   string
		resolved bool
	)
	if err := rows.Scan(&c.ID, &c.Author, &created, &updated, &anchor, &c.Body, &resolved); err != nil {
		return comments.Comment{}, err
	}
	c.Resolved = resolved

	t, err := time.Parse(timeFmt, created)
	if err != nil {
		return comments.Comment{}, fmt.Errorf("parse created_at of %q: %w", c.ID, err)
	}
	c.CreatedAt = t

	if updated != nil {
		u, err := time.Parse(timeFmt, *updated)
		if err != nil {
			return comments.Comment{}, fmt.Errorf("parse updated_at of %q: %w", c.ID, err)
		}
		c.UpdatedAt = u
	}
	if err := json.Unmarshal([]byte(anchor), &c.Anchor); err != nil {
		return comments.Comment{}, fmt.Errorf("decode anchor of %q: %w", c.ID, err)
	}
	return c, nil
}
