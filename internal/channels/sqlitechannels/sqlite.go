// Package sqlitechannels is the SQLite [channels.Store]: the sqlchannels
// implementation over the *sql.DB that sqlitedb owns.
//
// Channels live in rela.db for the reason comments do (see the
// sqlitecomments package doc): a copied or shipped database carries the
// conversation with the entities it is about.
//
// The sqlite tier is single-process (DEC-LFSYNY), so writes serialize on an
// in-process mutex and then on SQLite's own write lock, taken up front with
// BEGIN IMMEDIATE for the reason the sqlitedb package doc gives.
package sqlitechannels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
)

// DB is the database handle: the *sql.DB of sqlitedb.DB in production.
type DB interface {
	execer
	Conn(ctx context.Context) (*sql.Conn, error)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// New returns a channel store over db.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DB) (*sqlchannels.Store, error) {
	if db == nil {
		return nil, errors.New("sqlitechannels: New requires a database handle")
	}
	return sqlchannels.New(&handle{querier: querier{db}, db: db}, sqlchannels.Dialect{Name: "sqlitechannels"})
}

type handle struct {
	querier
	db      DB
	writeMu sync.Mutex
}

// Transact runs fn in a BEGIN IMMEDIATE transaction on a pinned connection. The
// deferred ROLLBACK and the uncancellable COMMIT keep a connection from
// returning to the pool with a transaction open, for the reason
// sqlitestore's Tx gives.
func (h *handle) Transact(ctx context.Context, fn func(sqlchannels.Querier) error) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	conn, err := h.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
		_ = conn.Close()
	}()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(querier{conn}); err != nil {
		return err
	}
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), "COMMIT"); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

// querier adapts database/sql to [sqlchannels.Querier].
type querier struct{ e execer }

func (q querier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := q.e.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (q querier) Query(ctx context.Context, query string, args ...any) (sqlchannels.Rows, error) {
	// The caller checks Err through the sqlchannels.Rows it gets back.
	rows, err := q.e.QueryContext(ctx, query, args...) //nolint:rowserrcheck // checked by the caller
	if err != nil {
		return nil, err
	}
	return sqlRows{rows}, nil
}

func (q querier) QueryRow(ctx context.Context, query string, args ...any) sqlchannels.Row {
	return sqlRow{q.e.QueryRowContext(ctx, query, args...)}
}

type sqlRows struct{ *sql.Rows }

func (r sqlRows) Close() { _ = r.Rows.Close() }

type sqlRow struct{ r *sql.Row }

func (r sqlRow) Scan(dest ...any) error {
	err := r.r.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return sqlchannels.ErrNoRows
	}
	return err
}
