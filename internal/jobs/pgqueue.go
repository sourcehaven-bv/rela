//go:build postgres

package jobs

import (
	"context"
	"fmt"
	"log/slog"

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
		return nil, fmt.Errorf("jobs: logger must not be nil")
	}
	if databaseURL == "" {
		return nil, fmt.Errorf("jobs: database URL must not be empty")
	}
	nq, err := neoq.New(ctx,
		neoq.WithBackend(postgres.Backend),
		postgres.WithConnectionString(databaseURL),
	)
	if err != nil {
		// The DSN carries credentials, so it must never reach the error
		// string — neoq's own errors are safe, ours must stay that way.
		return nil, fmt.Errorf("jobs: init postgres backend: %w", err)
	}
	return newNeoqQueue(nq, logger, DefaultQueueName, pgConcurrency)
}
