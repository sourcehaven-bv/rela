//go:build !postgres

package appbuild

import (
	"context"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// jobQueueShutdownTimeout bounds how long Close waits for the job queue to
// stop. A queue that will not drain must not wedge process shutdown.
const jobQueueShutdownTimeout = 5 * time.Second

// jobQueueFor returns the background-job queue for the default (fs/memory)
// build: in-process and EPHEMERAL — queued work is lost when the process exits.
//
// That is the correct semantic for this tier, not a limitation to work around.
// These are single-user local deployments; an unsent mail from a session that
// has ended is not worth resurrecting, and persisting it would mean a desktop
// app quietly replaying yesterday's side effects on launch.
//
// Durability arrives with the postgres build, where the deployment is a
// long-lived networked server — see jobqueue_postgres.go.
//
// base is unused here — the memory backend needs no configuration — but is
// taken so both build variants share one signature.
func jobQueueFor(_ *SharedBase) (jobs.Queue, error) {
	// slog.Default matches how the rest of appbuild logs; Config carries no
	// logger of its own.
	return jobs.NewMemoryQueue(context.Background(), slog.Default())
}
