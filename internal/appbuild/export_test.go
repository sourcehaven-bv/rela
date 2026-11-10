package appbuild

import (
	"context"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// FlowControlOver is [FlowControlFor] with its reads going to st, which
// wraps s's store. Test-only: it lets a budget test count the store reads
// a flow call makes.
func FlowControlOver(s *Services, st store.Store) *FlowControl {
	reader, _, holders := scriptReads(st, s.aclDeclarative, s.fieldReads, s.worlds)
	return &FlowControl{
		eng: s.flowEngine, d: s.aclDeclarative, rows: reader, holders: holders, raw: st,
		writer: entitymanager.FieldGated(s.entityManager), now: time.Now,
	}
}

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

// DetachChannelHintsForTest detaches s's hint endpoint, so its sends fail
// fast as a backend without hints. Test-only: the benchmark compares the two.
func DetachChannelHintsForTest(s *Services) {
	if s.channelHints != nil {
		s.channelHints.detach(s.hintAttach)
	}
}
