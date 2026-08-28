//go:build postgres

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/acaloiaro/neoq"
	"github.com/acaloiaro/neoq/backends/postgres"
)

// pgConcurrency is the worker count for a postgres queue.
//
// Higher than the memory tier: this is the networked deployment, where jobs
// are I/O-bound against external systems and the process is not sharing a
// machine with a user's desktop session. Each worker holds a connection while
// running, so this also bounds the queue's share of the pool.
const pgConcurrency = 10

// NewPostgresQueue returns a DURABLE queue backed by PostgreSQL: jobs survive
// process restarts and are safe to process from several rela-server processes
// against one database.
//
// It takes a DSN rather than rela's injected pgx pool, so it opens a SECOND
// pool. That departs from the pool-injection rule the store follows, and it is
// deliberate (TKT-YOED3R): neoq's postgres backend currently exposes only
// WithConnectionString. Upstream already has the field and the `if p.pool ==
// nil` guard needed for injection, so a WithPgxPool option is a small change to
// land later; revisit this when it does, or sooner if the extra pool causes
// connection-count pressure.
//
// The DSN comes from RELA_DATABASE_URL via appbuild.Config — never a flag, so
// the credential stays out of ps output and shell history.
//
// Nil: rejected — a nil logger is a wiring bug.
func NewPostgresQueue(ctx context.Context, logger *slog.Logger, databaseURL string) (Queue, error) {
	if logger == nil {
		return nil, errors.New("jobs: logger must not be nil")
	}
	if databaseURL == "" {
		return nil, errors.New("jobs: database URL must not be empty")
	}
	dsn := ensurePoolFloor(databaseURL, pgConcurrency+poolHeadroom)

	nq, err := neoq.New(ctx,
		neoq.WithBackend(postgres.Backend),
		postgres.WithConnectionString(dsn),
	)
	if err != nil {
		// The DSN carries credentials, so it must never reach the error
		// string — neoq's own errors are safe, ours must stay that way.
		return nil, fmt.Errorf("jobs: init postgres backend: %w", err)
	}
	return newNeoqQueue(nq, logger, pgConcurrency)
}

// poolHeadroom is the spare connections above the worker count.
//
// Workers hold a connection for as long as their handler runs, so a pool sized
// exactly to pgConcurrency leaves nothing for an Enqueue arriving while every
// worker is busy — it blocks for the full acquire timeout and then fails a
// caller that did nothing wrong. The headroom covers enqueues and neoq's own
// periodic queries.
const poolHeadroom = 4

// ensurePoolFloor returns dsn with pool_max_conns raised to at least floor.
//
// pgx derives its default pool size from GOMAXPROCS (min 4), which has nothing
// to do with how many workers this queue runs. On a 2-core machine that is 4
// connections against pgConcurrency=10 workers, so a handful of slow handlers
// starve every subsequent Enqueue — surfacing as "exceeded timeout acquiring a
// connection from the pool" under load, which reads like a database problem
// rather than a sizing one. Sizing the floor to the worker count makes the
// queue's connection demand a property of the queue, not of the host it lands
// on.
//
// An operator's explicit pool_max_conns is respected when it is already large
// enough, and only raised when it would deadlock the workers.
//
//nolint:unparam // floor is a parameter so the tests can vary it; the single production caller passing one value is the point, not an accident.
func ensurePoolFloor(dsn string, floor int) string {
	u, err := url.Parse(dsn)
	if err != nil {
		// Not a URL — a key/value DSN ("host=... dbname=..."), which pgx also
		// accepts. Leave it untouched rather than corrupting it; an operator
		// using that form can set pool_max_conns themselves.
		return dsn
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return dsn
	}

	q := u.Query()
	if cur := q.Get("pool_max_conns"); cur != "" {
		n, convErr := strconv.Atoi(cur)
		if convErr == nil && n >= floor {
			return dsn
		}
	}
	q.Set("pool_max_conns", strconv.Itoa(floor))
	u.RawQuery = q.Encode()
	return u.String()
}
