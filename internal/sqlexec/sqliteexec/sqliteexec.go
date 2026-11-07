// Package sqliteexec adapts a database/sql SQLite handle to [sqlexec.DB].
// It imports no driver: the sqlite recipe hands it the *sql.DB that
// sqlitedb owns.
//
// The sqlite tier is single-process (DEC-LFSYNY), so writes serialize on
// an in-process mutex per handle and then on SQLite's own write lock,
// taken up front with BEGIN IMMEDIATE for the reason the sqlitedb package
// doc gives.
package sqliteexec

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
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

// New returns db as a [sqlexec.DB] whose transactions are BEGIN IMMEDIATE
// on a pinned connection.
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DB) (sqlexec.DB, error) {
	if db == nil {
		return nil, errors.New("sqliteexec: New requires a database handle")
	}
	return &handle{querier: querier{db}, db: db}, nil
}

// NewInTx returns a [sqlexec.DB] whose writes join the transaction open on
// conn: each Transact runs in the named SAVEPOINT on conn, so it commits
// or rolls back with that transaction. It is for a write that must change
// the entity graph and another table atomically; conn is the one
// sqlitestore.TxConn returns inside a store transaction. The handle is
// valid only until that transaction ends. savepoint is a fixed SQL
// identifier the caller owns, never input; a name outside
// `^[a-z][a-z0-9_]{0,63}$` is refused, since it is spliced into SQL.
//
// Nil: rejected, so a wiring mistake fails at construction.
func NewInTx(conn *sql.Conn, savepoint string) (sqlexec.DB, error) {
	if conn == nil {
		return nil, errors.New("sqliteexec: NewInTx requires a connection")
	}
	if !savepointName.MatchString(savepoint) {
		return nil, fmt.Errorf("sqliteexec: savepoint name %q is not a plain identifier", savepoint)
	}
	return joined{querier: querier{conn}, conn: conn, savepoint: savepoint}, nil
}

var savepointName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// joined runs each write in a savepoint of a transaction someone else owns.
// It takes no write mutex: the owner already holds SQLite's write lock.
type joined struct {
	querier
	conn      *sql.Conn
	savepoint string
}

// Transact runs fn in a savepoint, and rolls back to it when fn fails.
func (j joined) Transact(ctx context.Context, fn func(sqlexec.Querier) error) error {
	if _, err := j.conn.ExecContext(ctx, "SAVEPOINT "+j.savepoint); err != nil {
		return fmt.Errorf("savepoint: %w", err)
	}
	if err := fn(j.querier); err != nil {
		undo := context.WithoutCancel(ctx)
		_, _ = j.conn.ExecContext(undo, "ROLLBACK TO "+j.savepoint)
		_, _ = j.conn.ExecContext(undo, "RELEASE "+j.savepoint)
		return err
	}
	if _, err := j.conn.ExecContext(context.WithoutCancel(ctx), "RELEASE "+j.savepoint); err != nil {
		return fmt.Errorf("release savepoint: %w", err)
	}
	return nil
}

type handle struct {
	querier
	db      DB
	writeMu sync.Mutex
}

// Transact runs fn in a BEGIN IMMEDIATE transaction on a pinned connection.
// The deferred ROLLBACK and the uncancellable COMMIT keep a connection from
// returning to the pool with a transaction open, for the reason
// sqlitestore's Tx gives.
func (h *handle) Transact(ctx context.Context, fn func(sqlexec.Querier) error) error {
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

// querier adapts database/sql to [sqlexec.Querier].
type querier struct{ e execer }

func (q querier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := q.e.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (q querier) Query(ctx context.Context, query string, args ...any) (sqlexec.Rows, error) {
	// The caller checks Err through the sqlexec.Rows it gets back.
	rows, err := q.e.QueryContext(ctx, query, args...) //nolint:rowserrcheck // checked by the caller
	if err != nil {
		return nil, err
	}
	return sqlRows{rows}, nil
}

func (q querier) QueryRow(ctx context.Context, query string, args ...any) sqlexec.Row {
	return sqlRow{q.e.QueryRowContext(ctx, query, args...)}
}

type sqlRows struct{ *sql.Rows }

func (r sqlRows) Close() { _ = r.Rows.Close() }

type sqlRow struct{ r *sql.Row }

func (r sqlRow) Scan(dest ...any) error {
	err := r.r.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return sqlexec.ErrNoRows
	}
	return err
}
