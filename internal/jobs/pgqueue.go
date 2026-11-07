//go:build postgres

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acaloiaro/neoq"
	"github.com/acaloiaro/neoq/backends/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
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
		postgres.WithTransactionTimeout(int(pgIdleTxTimeout.Milliseconds())),
	)
	if err != nil {
		// The DSN carries credentials, so it must never reach the error
		// string — neoq's own errors are safe, ours must stay that way.
		return nil, fmt.Errorf("jobs: init postgres backend: %w", err)
	}
	inner, err := newNeoqQueue(nq, logger, pgConcurrency)
	if err != nil {
		return nil, err
	}
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		nq.Shutdown(ctx)
		return nil, errors.New("jobs: parse database URL for the retention sweep")
	}
	poolCfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		nq.Shutdown(ctx)
		return nil, fmt.Errorf("jobs: open retention sweep pool: %w", err)
	}
	return &pgQueue{neoqQueue: inner, pool: pool, quit: make(chan struct{}), done: make(chan struct{})}, nil
}

// finishedJobRetention is how long the durable queue keeps the row of a
// finished job: a row in neoq_jobs that neoq marked processed, and a row in
// neoq_dead_jobs. neoq itself never deletes either, so without the sweep a
// payload would stay in the database for good (TKT-RJS3KKJ). A day leaves an
// operator time to inspect a failure; nothing in rela reads the rows.
//
// Pending rows, including failed ones awaiting a retry, are never swept.
const finishedJobRetention = 24 * time.Hour

// retentionSweepInterval is how often the sweep runs. The first sweep
// waits one interval too, so a short-lived process such as a CLI command
// exits before it would start one.
const retentionSweepInterval = 10 * time.Minute

// sweepBatch bounds the rows one DELETE removes, so a large backlog, such
// as the rows of a deployment that ran before the sweep existed, is removed
// in short statements rather than one long one.
const sweepBatch = 1000

// pgQueue is the neoq queue plus the retention sweep of its tables. The
// sweep has a pool of one connection of its own, because neoq does not
// expose its pool.
type pgQueue struct {
	*neoqQueue
	pool *pgxpool.Pool

	stopOnce  sync.Once
	quit      chan struct{} // closed by Close
	done      chan struct{} // closed when the sweep has ended
	sweepOnce sync.Once
}

// Start starts the queue and then the retention sweep.
func (q *pgQueue) Start(ctx context.Context) error {
	if err := q.neoqQueue.Start(ctx); err != nil {
		return err
	}
	// The sweep outlives Start's ctx; Close stops it.
	q.sweepOnce.Do(func() { go q.sweepLoop() }) //nolint:contextcheck // see above
	return nil
}

// Close cancels the sweep and closes the queue. It waits for a running
// sweep only until ctx ends.
func (q *pgQueue) Close(ctx context.Context) error {
	q.stopOnce.Do(func() {
		close(q.quit)
		// Uses up sweepOnce when the sweep never started, so it cannot
		// start now, and there is nothing to wait for.
		q.sweepOnce.Do(func() { close(q.done) })
	})
	select {
	case <-q.done:
		q.pool.Close()
	case <-ctx.Done():
		// The cancelled sweep returns soon; its pool goes with the process.
	}
	return q.neoqQueue.Close(ctx)
}

func (q *pgQueue) sweepLoop() {
	defer close(q.done)
	stopCtx, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		select {
		case <-q.quit:
			stop()
		case <-stopCtx.Done():
		}
	}()
	t := time.NewTicker(retentionSweepInterval)
	defer t.Stop()
	for {
		select {
		case <-stopCtx.Done():
			return
		case <-t.C:
		}
		ctx, cancel := context.WithTimeout(stopCtx, time.Minute)
		if n, err := q.sweepFinished(ctx, time.Now().Add(-finishedJobRetention)); err != nil {
			if stopCtx.Err() == nil {
				q.logger.Warn("jobs: retention sweep failed", "error", err)
			}
		} else if n > 0 {
			q.logger.Debug("jobs: retention sweep removed finished jobs", "rows", n)
		}
		cancel()
	}
}

// sweepFinished deletes this queue's finished and dead job rows that ended
// before cutoff, in batches of sweepBatch, and returns how many it deleted.
// A batch that commits stays deleted if a later one fails.
func (q *pgQueue) sweepFinished(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for _, stmt := range []string{
		`DELETE FROM neoq_jobs WHERE id IN (SELECT id FROM neoq_jobs
			WHERE queue = $1 AND status = 'processed' AND ran_at < $2 LIMIT $3)`,
		`DELETE FROM neoq_dead_jobs WHERE ctid IN (SELECT ctid FROM neoq_dead_jobs
			WHERE queue = $1 AND created_at < $2 LIMIT $3)`,
	} {
		for {
			tag, err := q.pool.Exec(ctx, stmt, queueName, cutoff, sweepBatch)
			if err != nil {
				return total, fmt.Errorf("retention sweep: %w", err)
			}
			total += tag.RowsAffected()
			if tag.RowsAffected() < sweepBatch {
				break
			}
		}
	}
	return total, nil
}

// pgIdleTxTimeout is the idle_in_transaction_session_timeout neoq sets on
// every worker connection.
//
// neoq runs a handler INSIDE the transaction that holds the job's row lock and
// records the outcome in that same transaction. While the handler runs, the
// transaction is idle as far as PostgreSQL is concerned. neoq's default for
// this timeout is 30 s, so any handler longer than that had its session killed
// underneath it: the outcome update failed, the row stayed pending, neoq
// redelivered it every minute forever, and its idempotency key rejected every
// later enqueue (BUG-TKL08E).
//
// Derived from handlerTimeout so a handler that stays within its own timeout
// can always commit. The margin covers the status update itself.
const pgIdleTxTimeout = handlerTimeout + time.Minute

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
// Only the pool_max_conns parameter is rewritten; every other parameter keeps
// the operator's own bytes. Setting it through url.Values and re-encoding the
// whole query would rewrite the others into Go's encoding, and Go encodes a
// space as "+" where a URI query means a literal plus. libpq reads it that way,
// and so does pgx from v5.11.0 on, so a DSN carrying
// `options=-c%20search_path%3Dtenant` came back out as `-c+search_path=tenant`
// and the server rejected the parameter name "+search_path" — a working DSN
// turned into a connection failure by a helper that only meant to size a pool.
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

	want := "pool_max_conns=" + strconv.Itoa(floor)
	if cur := u.Query().Get("pool_max_conns"); cur != "" {
		n, convErr := strconv.Atoi(cur)
		if convErr == nil && n >= floor {
			return dsn
		}
		// Too small, or unparseable — either way the workers need the floor.
		u.RawQuery = replaceRawParam(u.RawQuery, "pool_max_conns", want)
		return u.String()
	}

	if u.RawQuery == "" {
		u.RawQuery = want
	} else {
		u.RawQuery += "&" + want
	}
	return u.String()
}

// replaceRawParam substitutes the key=value pair for key in a raw query string,
// leaving the surrounding pairs byte-for-byte as they were. See ensurePoolFloor
// for why re-encoding the whole query is not an option.
func replaceRawParam(raw, key, pair string) string {
	parts := strings.Split(raw, "&")
	for i, p := range parts {
		if p == key || strings.HasPrefix(p, key+"=") {
			parts[i] = pair
		}
	}
	return strings.Join(parts, "&")
}
