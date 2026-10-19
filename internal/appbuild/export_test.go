package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// HasVersionSweep reports whether s assembled a version-reconciliation sweep.
// Test-only: it lets the external tests check the wiring through New without
// widening Services.
func HasVersionSweep(s *Services) bool { return s.versionSweep != nil }

// PromoterFor returns the promoter assembly wires into s's channel service,
// and whether it commits atomically. Test-only: it lets the external tests
// drive a promote whose mark fails, which the service cannot reach.
func PromoterFor(s *Services) (p interface {
	Promote(ctx context.Context, r realtime.PromoteRequest) (realtime.PromoteResult, error)
}, atomic bool, err error) {
	mp, err := newManagerPromoter(s.entityManager, txChannelsFor(s.store, s.channels))
	return mp, mp.join != nil, err
}
