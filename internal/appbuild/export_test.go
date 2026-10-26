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

// OutputsFor returns the session outputs writer assembly wires into s's
// channel service. join false drops the channel store's join, as a wiring
// whose stores do not share a database has. Test-only.
func OutputsFor(s *Services, join bool) (realtime.OutputWriter, error) {
	j := txChannelsFor(s.store, s.channels)
	if !join {
		j = nil
	}
	return newManagerOutputs(s.entityManager, s.store, s.meta, j, s.aclDeclarative)
}
