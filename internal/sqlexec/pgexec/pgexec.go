// Package pgexec adapts a pgx handle to [sqlexec.DB]: it numbers the `?`
// placeholders and runs transactions on the pool the composition root
// shares with pgstore.
package pgexec

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// DBTX is the database handle: a *pgxpool.Pool in production. Restated
// rather than imported from pgstore, for the reason pgcomments.DBTX gives.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// New returns db as a [sqlexec.DB].
//
// Nil: rejected, so a wiring mistake fails at construction.
func New(db DBTX) (sqlexec.DB, error) {
	if db == nil {
		return nil, errors.New("pgexec: New requires a database handle")
	}
	return handle{querier{db}, db}, nil
}

type handle struct {
	querier
	db DBTX
}

// Transact runs fn in one transaction. Rollback and commit run
// uncancellable, so a request that goes away mid-write still releases its
// connection cleanly.
func (h handle) Transact(ctx context.Context, fn func(sqlexec.Querier) error) error {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(querier{tx}); err != nil {
		return err
	}
	if err := tx.Commit(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// querier adapts pgx to [sqlexec.Querier], numbering the placeholders.
type querier struct{ e execer }

// rebind turns the `?` placeholders into `$1`, `$2`, ... The queries of
// the shared backends hold no `?` inside a literal.
func rebind(q string) string {
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (q querier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	tag, err := q.e.Exec(ctx, rebind(query), args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (q querier) Query(ctx context.Context, query string, args ...any) (sqlexec.Rows, error) {
	rows, err := q.e.Query(ctx, rebind(query), args...)
	if err != nil {
		return nil, err
	}
	return pgRows{rows}, nil
}

func (q querier) QueryRow(ctx context.Context, query string, args ...any) sqlexec.Row {
	return pgRow{q.e.QueryRow(ctx, rebind(query), args...)}
}

type pgRows struct{ pgx.Rows }

func (r pgRows) Close() { r.Rows.Close() }

type pgRow struct{ r pgx.Row }

func (r pgRow) Scan(dest ...any) error {
	err := r.r.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlexec.ErrNoRows
	}
	return err
}
