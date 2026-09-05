// Package statesql stores rela's runtime state — the document render cache,
// user settings, the operator's logo and theme, the CalDAV alias table — in a
// SQL table rather than in files under .rela/.
//
// It exists for the same reason [configsql] does, and is deliberately its
// sibling rather than part of a storage backend: none of this is the entity
// graph. It takes a *sql.DB the caller already opened and reads rows keyed by
// string.
//
// The motivation differs from the postgres backend's, which moved this state
// into the database because several rela-server processes share one database
// and node-local files meant an uploaded logo was served by exactly one of
// them. SQLite is single-process, so that is not the problem here. The problem
// is that a state file under .rela/ sits BESIDE the database rather than
// inside it — so a shipped single file would arrive with its palette, logo and
// user settings silently left behind.
package statesql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"
)

// maxValueBytes caps a stored value. Matching the postgres backend's limit
// rather than picking a new one: the cap exists so a runaway render cannot
// wedge the database, and a value that would be truncated must be REJECTED
// rather than stored short — a silently truncated cached render would be
// served as if it were valid.
const maxValueBytes = 32 << 20 // 32 MiB

// timeFmt is the on-disk timestamp format, matching what the store writes to
// its own rows: RFC3339Nano, timezone retained.
const timeFmt = time.RFC3339Nano

// KV is the SQL-backed durable key/value store.
//
// It implements state.KV, but does NOT name that interface here: importing
// internal/state would make this package depend on the one that wraps it, and
// the wiring site already binds the two. The conformance suite
// (state/statetest) is what proves the match, which is stronger than a
// compile-time assertion because it checks behaviour, not just shape.
//
// # Key policy lives with the caller
//
// This type stores whatever key it is given. Key VALIDATION — rejecting
// traversal, absolute paths, Windows-hostile names — is the state package's
// contract, enforced by state.ValidatedKV wrapping this. One implementation of
// those rules, rather than a copy here that could drift from the filesystem
// backend's.
//
// Nil: [New] rejects a nil handle.
type KV struct {
	db *sql.DB
}

// New returns a KV over db.
//
// The caller owns db: this borrows the handle and never closes it.
func New(db *sql.DB) (*KV, error) {
	if db == nil {
		return nil, errors.New("statesql: a non-nil database handle is required")
	}
	return &KV{db: db}, nil
}

// Get returns the value stored at key.
//
// Errors: a missing key returns an [fs.ErrNotExist]-compatible error, which is
// the contract every consumer branches on — the scheduler reading a never-set
// last-run timestamp treats it as a normal state, not a failure.
func (k *KV) Get(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := k.db.QueryRowContext(ctx,
		`SELECT value FROM state_kv WHERE key = ?`, key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &fs.PathError{Op: "open", Path: key, Err: fs.ErrNotExist}
	}
	if err != nil {
		return nil, fmt.Errorf("statesql: read %q: %w", key, err)
	}
	return value, nil
}

// Put stores value at key, replacing whatever was there.
func (k *KV) Put(ctx context.Context, key string, value []byte) error {
	if len(value) > maxValueBytes {
		return fmt.Errorf(
			"statesql: value for %q is %d bytes, over the %d-byte limit",
			key, len(value), maxValueBytes)
	}
	_, err := k.db.ExecContext(ctx,
		`INSERT INTO state_kv (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value,
		                                updated_at = excluded.updated_at`,
		key, value, time.Now().UTC().Format(timeFmt))
	if err != nil {
		return fmt.Errorf("statesql: write %q: %w", key, err)
	}
	return nil
}

// Delete removes key.
//
// A missing key is not an error: delete is idempotent, so a caller clearing
// state it may or may not have written does not have to check first.
func (k *KV) Delete(ctx context.Context, key string) error {
	if _, err := k.db.ExecContext(ctx, `DELETE FROM state_kv WHERE key = ?`, key); err != nil {
		return fmt.Errorf("statesql: delete %q: %w", key, err)
	}
	return nil
}
