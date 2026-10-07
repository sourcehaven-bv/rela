//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// SQLiteData is every store rela keeps inside one sqlite database file: the
// entity graph, the runtime state KV, comments and the applied-migration
// record.
//
// It exists for `rela db import-fs`, which fills a new database without
// building a full [Services]: no search index, no metamodel, no ACL. The
// stores come from the same [openDBServices] the server recipe uses, so a
// table added there is picked up here too.
type SQLiteData struct {
	Store      store.Store
	State      state.KV
	Comments   comments.Store
	Migrations datamigration.StateStore

	db *sqlitedb.DB
}

// OpenSQLiteData opens (and creates, when missing) the database at path.
//
// The caller owns the result and must Close it. Opening takes the database's
// single-writer lock, so it fails while another process has the file open.
func OpenSQLiteData(ctx context.Context, path string) (*SQLiteData, error) {
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
	if err != nil {
		return nil, err
	}
	st, err := sqlitestore.New(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	svc, err := openDBServices(db)
	if err != nil {
		_ = st.Close()
		_ = db.Close()
		return nil, err
	}
	return &SQLiteData{
		Store:      st,
		State:      svc.kv,
		Comments:   svc.comments,
		Migrations: svc.migState,
		db:         db,
	}, nil
}

// Checkpoint copies the write-ahead log into the database file and truncates
// it, so the file is complete on its own once the handle is closed.
func (d *SQLiteData) Checkpoint(ctx context.Context) error {
	if _, err := d.db.DB().ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("appbuild: checkpoint: %w", err)
	}
	return nil
}

// Close stops the store and releases the database.
func (d *SQLiteData) Close() error {
	return errors.Join(d.Store.Close(), d.db.Close())
}

// Tables lists the database's own tables by name, without SQLite's internal
// ones. It lets a copy into the database check that it accounts for every
// table.
func (d *SQLiteData) Tables(ctx context.Context) ([]string, error) {
	rows, err := d.db.DB().QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("appbuild: list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("appbuild: list tables: %w", err)
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
