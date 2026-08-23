package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/acaloiaro/neoq"
	"github.com/acaloiaro/neoq/backends/memory"
)

// DefaultQueueName is the neoq queue rela submits every job to. Kinds are
// routed within it by the dispatch handler — see neoqQueue.
const DefaultQueueName = "rela"

// defaultConcurrency is the worker count for a memory queue.
//
// Small on purpose. These jobs are I/O-bound against external systems, and the
// low tier is a single-user desktop app where a wide pool would compete with
// the UI for no throughput gain.
const defaultConcurrency = 4

// NewMemoryQueue returns an in-process, EPHEMERAL queue: jobs live in memory
// and are lost when the process exits.
//
// That is the correct semantic for the fs/desktop tier, not a degraded one —
// an unsent mail from a session that has ended is not worth resurrecting.
// Callers needing durability wire the postgres backend instead (see
// NewPostgresQueue on the postgres build).
//
// This constructor deliberately links no database driver, which is what keeps
// the default build free of pgx.
//
// Nil: rejected — a nil logger is a wiring bug.
func NewMemoryQueue(ctx context.Context, logger *slog.Logger) (Queue, error) {
	if logger == nil {
		return nil, errors.New("jobs: logger must not be nil")
	}
	nq, err := neoq.New(ctx, neoq.WithBackend(memory.Backend))
	if err != nil {
		return nil, fmt.Errorf("jobs: init memory backend: %w", err)
	}
	return newNeoqQueue(nq, logger, DefaultQueueName, defaultConcurrency)
}
