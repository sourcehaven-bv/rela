//go:build postgres

package jobs

import (
	"context"
	"time"
)

// SweepFinished runs one retention sweep of q, a queue from
// NewPostgresQueue, with the given cutoff.
func SweepFinished(ctx context.Context, q Queue, cutoff time.Time) (int64, error) {
	return q.(*pgQueue).sweepFinished(ctx, cutoff)
}
