package tenant

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// DueTenant is an active tenant with background work due at DueAt: a flow
// run, a channel session timer, or a queued job.
type DueTenant struct {
	OrgID string
	DueAt time.Time
}

// The sweep's defaults.
const (
	// SweepInterval is how often a host looks for evicted tenants with due
	// work. It bounds how late a timer of a tenant nobody opens fires.
	SweepInterval = time.Minute
	// sweepHold is how long the sweep keeps a tenant it opened. It covers
	// one flow worker tick and the job queue's first fetch.
	sweepHold = 30 * time.Second
	// sweepCooldown is how long the sweep leaves an org alone after opening
	// it, so a run that keeps failing does not reopen its tenant every pass.
	sweepCooldown = 10 * time.Minute
	// sweepMaxCooldown caps the cooldown of an org whose due work stays
	// due: each open that leaves the same due time doubles it.
	sweepMaxCooldown = 6 * time.Hour
	// sweepPerPass caps the tenants one pass opens, so a backlog of due
	// tenants does not push every active tenant out of the registry.
	sweepPerPass = 2
)

// sweepLister lists the active tenants whose earliest due work is at or
// before until.
type sweepLister interface {
	DueTenants(ctx context.Context, until time.Time) ([]DueTenant, error)
}

// sweepRegistry is what the sweep needs of a [Registry].
type sweepRegistry interface {
	IsResident(orgID string) bool
	acquireFor(ctx context.Context, orgID string) (release func(), err error)
}

// Sweeper opens tenants that are not resident but have background work due
// (TKT-ZOU5BCT, TKT-N3X235N).
//
// A tenant's workers run only while it is resident: the flow worker, the
// realtime timers and the durable job queue. A tenant nobody uses is
// evicted, so its timers would wait until someone opened it. The sweep
// reads one row per tenant per pass and opens a due one for [sweepHold], so
// its workers get a tick; the registry then evicts it as usual.
//
// It stays inside the registry's bounds: opening a tenant evicts an idle
// one as any request does, and a pass opens at most [sweepPerPass].
//
// Maintenance with no due time (soft-delete GC, the version sweep) is not
// swept: it runs the next time the tenant opens, and nothing waits on it.
type Sweeper struct {
	list sweepLister
	reg  sweepRegistry
	now  func() time.Time

	hold     time.Duration
	cooldown time.Duration
	perPass  int

	mu   sync.Mutex
	last map[string]sweepMark
	held sync.WaitGroup
}

// sweepMark is the sweep's last open of an org: when, for which due time,
// and how long the org is left alone after it.
type sweepMark struct {
	at, dueAt time.Time
	wait      time.Duration
}

// NewSweeper builds a sweeper over the tenants list reports and the
// registry reg.
//
// Nil: rejected — both are required.
func NewSweeper[R Resident](list sweepLister, reg *Registry[R]) (*Sweeper, error) {
	if list == nil || reg == nil {
		return nil, errors.New("tenant: sweeper needs a tenant lister and a registry")
	}
	return newSweeper(list, registrySweep[R]{reg}, time.Now), nil
}

func newSweeper(list sweepLister, reg sweepRegistry, now func() time.Time) *Sweeper {
	return &Sweeper{
		list: list, reg: reg, now: now,
		hold: sweepHold, cooldown: sweepCooldown, perPass: sweepPerPass,
		last: make(map[string]sweepMark),
	}
}

// Run sweeps every interval until ctx ends, then waits for the tenants it
// holds to be released.
func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	defer s.held.Wait()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.Pass(ctx)
		}
	}
}

// Pass runs one sweep and reports how many tenants it opened.
func (s *Sweeper) Pass(ctx context.Context) int {
	now := s.now()
	due, err := s.list.DueTenants(ctx, now)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("tenant sweep: listing due tenants failed", "error", err)
		}
		return 0
	}
	opened := 0
	for _, d := range due {
		if opened >= s.perPass || ctx.Err() != nil {
			break
		}
		if s.reg.IsResident(d.OrgID) || !s.mayOpen(d, now) {
			continue
		}
		release, err := s.reg.acquireFor(ctx, d.OrgID)
		if err != nil {
			slog.Warn("tenant sweep: opening a tenant with due work failed", "org_id", d.OrgID, "error", err)
			continue
		}
		slog.Info("tenant sweep: opened a tenant with due work", "org_id", d.OrgID, "due_at", d.DueAt)
		opened++
		s.held.Go(func() {
			defer release()
			select {
			case <-ctx.Done():
			case <-time.After(s.hold):
			}
		})
	}
	return opened
}

// mayOpen reports whether d's org is past its cooldown, and starts a new
// one when it is. When the last open left the same due time, the work did
// not clear, so the cooldown doubles up to [sweepMaxCooldown]. An entry is
// dropped once twice its cooldown has passed, so the map stays bounded by
// the orgs opened recently, and long enough to see the due time again.
func (s *Sweeper) mayOpen(d DueTenant, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for org, m := range s.last {
		if now.Sub(m.at) >= 2*m.wait {
			delete(s.last, org)
		}
	}
	m, ok := s.last[d.OrgID]
	if ok && now.Sub(m.at) < m.wait {
		return false
	}
	wait := s.cooldown
	if ok && m.dueAt.Equal(d.DueAt) {
		wait = min(2*m.wait, sweepMaxCooldown)
	}
	s.last[d.OrgID] = sweepMark{at: now, dueAt: d.DueAt, wait: wait}
	return true
}

// registrySweep adapts a [Registry] to the sweep, which does not care what
// a resident holds.
type registrySweep[R Resident] struct{ r *Registry[R] }

func (a registrySweep[R]) IsResident(orgID string) bool { return a.r.IsResident(orgID) }

func (a registrySweep[R]) acquireFor(ctx context.Context, orgID string) (func(), error) {
	l, err := a.r.Acquire(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return l.Release, nil
}
