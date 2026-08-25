//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// jobQueueFor returns the background-job queue for the postgres build: DURABLE
// work that survives a restart and is safe to process from several rela-server
// processes against one database.
//
// It opens its own connection pool from the DSN rather than sharing the pgx
// pool appbuild already owns, because neoq's postgres backend currently exposes
// only WithConnectionString. That is a deliberate, recorded departure from the
// pool-injection rule (TKT-YOED3R) and the reason to revisit it is
// connection-count pressure, not tidiness.
//
// With a DSN this is DURABLE and a failure to build it is fatal: a postgres
// deployment that quietly fell back to the ephemeral queue would look healthy
// while losing every queued job on restart.
//
// WITHOUT a DSN it falls back to the ephemeral queue, because the build tag
// does not imply a postgres STORE. The postgres binary assembles over an
// in-memory store in the composition-root tests, and Config.DatabaseURL is
// empty there; demanding a DSN from the tag alone failed those assemblies for
// a queue they never enqueue to. Durability follows the store, so the tier
// that has no database gets the queue that needs none — and an operator who
// set RELA_DATABASE_URL still gets the hard failure above if it is unusable.
func jobQueueFor(base *SharedBase) (jobs.Queue, error) {
	if base == nil {
		return nil, errors.New("appbuild: nil SharedBase")
	}
	if base.cfg.DatabaseURL == "" {
		return jobs.NewMemoryQueue(context.Background(), slog.Default())
	}
	return jobs.NewPostgresQueue(context.Background(), slog.Default(), base.cfg.DatabaseURL)
}
