package sqlitestore

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Test-only accessors. They live here so the sweep can be driven a tick at a
// time without widening the package's real API for it, and without a test
// having to wait out a ticker interval.

// SweepNow runs exactly one reconciliation tick synchronously.
func (s *Store) SweepNow(ctx context.Context, p store.ProjectionProvider, cfg store.SweepConfig) error {
	return s.sweepNow(ctx, p, cfg)
}
