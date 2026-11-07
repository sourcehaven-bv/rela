// Package sqlexec is the narrow statement interface the shared SQL
// backends (sqlchannels, sqlflowrun) are written against, so they import
// no driver. pgexec adapts pgx to it and sqliteexec adapts database/sql;
// a backend package picks one at its constructor.
package sqlexec

import (
	"context"
	"errors"
)

// ErrNoRows is what a [Row] returns from Scan when the query matched
// nothing. Adapters translate their driver's own sentinel to it.
var ErrNoRows = errors.New("sqlexec: no rows")

// Querier runs statements. Queries use `?` placeholders; an adapter whose
// driver numbers them rewrites the query, so a query holds no `?` other
// than a placeholder: no `'?'` literal and no PostgreSQL JSONB `?`
// operator.
type Querier interface {
	Exec(ctx context.Context, query string, args ...any) (rowsAffected int64, err error)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row
}

// Rows is a result set.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Row is a single-row result.
type Row interface {
	Scan(dest ...any) error
}

// DB is a database handle that runs transactions.
type DB interface {
	Querier
	// Transact runs fn in one transaction, committing when fn returns nil
	// and rolling back otherwise.
	Transact(ctx context.Context, fn func(q Querier) error) error
}
