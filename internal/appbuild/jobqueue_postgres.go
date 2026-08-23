//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// jobQueueShutdownTimeout bounds how long Close waits for the job queue to
// stop. A queue that will not drain must not wedge process shutdown.
const jobQueueShutdownTimeout = 5 * time.Second

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
// A missing DSN is a hard error, not a silent downgrade to the ephemeral queue:
// a postgres deployment that quietly stopped persisting jobs would look healthy
// while losing work on every restart.
func jobQueueFor(base *SharedBase) (jobs.Queue, error) {
	if base == nil {
		return nil, errors.New("appbuild: nil SharedBase")
	}
	return jobs.NewPostgresQueue(context.Background(), slog.Default(), base.cfg.DatabaseURL)
}
