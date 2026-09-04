package tenant

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

// DefaultMaxResident is the resident-set bound applied when an operator sets
// none.
//
// Small on purpose, because an open store is expensive in the one resource that
// runs out first. `pgstore.Open` builds a pgx pool with no MaxConns — so
// max(4, numCPU) — plus one dedicated non-pooled connection for LISTEN, plus
// the version sweep's own. RES-S8CH9C measured that at roughly 17 connections
// per open store on a 16-core box, and against a typical max_connections=100
// that is about five resident tenants, not five hundred.
//
// A default that quietly exceeded the cluster's connection budget would fail as
// "too many connections" under load — an outage affecting every tenant,
// triggered by traffic to any of them. Failing instead by evicting is a latency
// cost paid by one tenant.
const DefaultMaxResident = 4

// Opener opens the backing store for one tenant.
//
// Injected rather than calling `appbuild.New` directly, because the store a
// build opens is build-tag-dependent (PostgreSQL, filesystem, memory) while
// this package is not. It is also the seam the tests use to count opens and
// closes without a database.
//
// Implementations must return a usable [appbuild.Services] or an error, never
// both — the fail-closed rule from the package doc, applied one level down.
type Opener func(ctx context.Context, t Tenant) (*appbuild.Services, error)

// AppBuildOpener opens a tenant's store through appbuild, using base for
// everything that is tenant-independent.
//
// Note that it sets Config.DatabaseURL rather than passing
// appbuild.WithDatabaseURL: the option is consumed by appbuild.Discover, while
// appbuild.New — which is what a caller assembling a Config by hand reaches —
// reads the field. Passing the option here would compile, do nothing, and open
// every tenant against whichever DSN the config already held. That is precisely
// the cross-tenant failure this package exists to prevent, and it would be
// invisible until two tenants were resident at once.
func AppBuildOpener(base appbuild.Config) Opener {
	return func(_ context.Context, t Tenant) (*appbuild.Services, error) {
		cfg := base
		cfg.DatabaseURL = t.DSN
		//nolint:contextcheck // appbuild.New takes no ctx; it opens its own on the
		// build recipe's behalf. Opening a tenant's store must also outlive the
		// request that triggered it — a cancelled request must not leave a
		// half-migrated schema behind.
		return appbuild.New(cfg)
	}
}

// Lease is a borrowed reference to one tenant's services.
//
// It exists because eviction and use overlap. The resident set is bounded, so
// acquiring tenant F may evict tenant A — and if a request is still reading
// through A's store, closing it is a use-after-close, which in a pgx pool is a
// panic rather than an error. Counting references is the only way to know, and
// hoping request lifetimes are shorter than eviction pressure is not knowing.
//
// Every successful [Registry.Acquire] must be matched by exactly one
// [Lease.Release], conventionally deferred.
type Lease struct {
	// Tenant is the resolved tenant this lease serves, carried so a caller can
	// log or audit the tenant it acted as without a second lookup.
	Tenant Tenant

	svc      *appbuild.Services
	registry *Registry
	released bool
}

// Services returns the tenant-scoped services this lease borrows.
//
// The returned value is valid only until [Lease.Release]. It is scoped to
// exactly one tenant's schema: this is where the isolation guarantee is
// actually delivered, since every query issued through it resolves through a
// `search_path` that names one tenant's schema and no other's.
func (l *Lease) Services() *appbuild.Services { return l.svc }

// Release returns the lease. Safe to call more than once so a handler can
// defer it unconditionally; only the first call counts, because a double
// release would drop another caller's reference and re-open the
// use-after-close it exists to prevent.
func (l *Lease) Release() {
	if l == nil || l.released {
		return
	}
	l.released = true
	l.registry.release(l.Tenant.OrgID)
}

// resident is one tenant's open store and its bookkeeping.
type resident struct {
	tenant   Tenant
	svc      *appbuild.Services
	refs     int
	element  *list.Element // position in the registry's recency list
	evicting bool
}

// Registry hands out per-tenant services over one shared configuration.
//
// It is the multi-store host `appbuild.SharedBase` was built for: one parsed
// metamodel and one parsed ACL policy, assembled against N stores. Which is
// also the reason it is cheap to serve many tenants — under this design the
// tenants differ only in content, so the expensive half of construction happens
// once for all of them.
//
// # What it guarantees
//
//   - An org that does not resolve gets no store (see the package doc).
//   - At most MaxResident stores are open at once, independently of how many
//     tenants exist. That decoupling is the property that makes tenant count an
//     operations question instead of an architecture one.
//   - Evicting one tenant closes that tenant's store and nothing else. This is
//     inherited rather than implemented: Services.Close tears down only the
//     store and search closer it was assembled with, never anything owned by
//     the shared base.
//
// # What it does not do
//
// It never creates a schema. An unknown org is denied, not provisioned
// (TKT-TNPRV8), and no tenant is ever erased here (TKT-TNERAS).
type Registry struct {
	resolver    Resolver
	open        Opener
	maxResident int

	mu        sync.Mutex
	residents map[string]*resident
	recency   *list.List // front = most recently used
}

// NewRegistry builds a registry over a resolver and an opener.
//
// maxResident <= 0 takes [DefaultMaxResident]. Both collaborators are required
// and nil is rejected at construction rather than at first request, per
// CLAUDE.md — and here for a sharper reason than usual: a registry that failed
// only on use would fail inside a request, where the natural recovery is to
// serve the request some other way.
func NewRegistry(resolver Resolver, open Opener, maxResident int) (*Registry, error) {
	if resolver == nil {
		return nil, errors.New("tenant.NewRegistry: resolver is required")
	}
	if open == nil {
		return nil, errors.New("tenant.NewRegistry: opener is required")
	}
	if maxResident <= 0 {
		maxResident = DefaultMaxResident
	}
	return &Registry{
		resolver:    resolver,
		open:        open,
		maxResident: maxResident,
		residents:   make(map[string]*resident),
		recency:     list.New(),
	}, nil
}

// Acquire resolves orgID and leases its services, opening the store if the
// tenant is not resident.
//
// Returns an error and a nil lease for any org it cannot resolve — including
// the empty org — so a caller cannot accidentally proceed with a usable handle.
// Check the error; do not check the lease for nil and carry on.
//
// The caller must Release the returned lease.
func (r *Registry) Acquire(ctx context.Context, orgID string) (*Lease, error) {
	t, err := r.resolver.Resolve(orgID)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	if res, ok := r.residents[t.OrgID]; ok && !res.evicting {
		res.refs++
		r.recency.MoveToFront(res.element)
		r.mu.Unlock()
		return &Lease{Tenant: res.tenant, svc: res.svc, registry: r}, nil
	}
	r.mu.Unlock()

	// Open outside the lock: opening a store connects to a database and runs
	// migrations, and holding the registry lock across that would stall every
	// other tenant's requests behind one tenant's cold start.
	//
	// The cost is that concurrent first requests for the same tenant can open
	// more than one store. That is wasteful, not incorrect — the loser is
	// closed below, and pgstore.Migrate is already safe to start concurrently,
	// serializing under an advisory lock keyed on the schema. Single-flighting
	// it belongs with the provisioning work (TKT-TNPRV8), which has to
	// single-flight schema creation anyway and should own one mechanism rather
	// than two.
	svc, err := r.open(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("tenant %q: open store: %w", t.OrgID, err)
	}
	if svc == nil {
		// An opener returning (nil, nil) would put a nil store into the
		// resident set and hand it to a request. Refuse it here: this is the
		// zero-value-means-allowed shape the package doc warns about, and the
		// only place it could enter.
		return nil, fmt.Errorf("tenant %q: opener returned no services", t.OrgID)
	}

	r.mu.Lock()
	if res, ok := r.residents[t.OrgID]; ok && !res.evicting {
		// Another goroutine won the race. Use its store and close ours.
		res.refs++
		r.recency.MoveToFront(res.element)
		r.mu.Unlock()
		closeServices(t, svc)
		return &Lease{Tenant: res.tenant, svc: res.svc, registry: r}, nil
	}
	res := &resident{tenant: t, svc: svc, refs: 1}
	res.element = r.recency.PushFront(t.OrgID)
	r.residents[t.OrgID] = res
	evicted := r.evictLocked()
	r.mu.Unlock()

	// Eviction must NOT inherit this request's context. The tenant being closed
	// is a different tenant from the one being served, so canceling the
	// request must not abandon another tenant's pool half-torn-down.
	closeAll(evicted)
	return &Lease{Tenant: t, svc: svc, registry: r}, nil
}

// release drops one reference, closing the store if the tenant was already
// evicted and this was the last holder.
func (r *Registry) release(orgID string) {
	r.mu.Lock()
	res, ok := r.residents[orgID]
	if !ok {
		r.mu.Unlock()
		return
	}
	res.refs--
	var closing *resident
	if res.refs <= 0 && res.evicting {
		delete(r.residents, orgID)
		closing = res
	}
	r.mu.Unlock()

	if closing != nil {
		closeServices(closing.tenant, closing.svc)
	}
}

// evictLocked trims the resident set to the bound, returning the residents
// whose stores the caller must close. Requires r.mu.
//
// Only unreferenced residents are closed here. One still in use is marked
// evicting instead: it leaves the recency list and the bound immediately, and
// its last [Lease.Release] closes it. So the bound is a bound on *idle* stores,
// and a burst of concurrent tenants can exceed it briefly rather than closing a
// store a request is reading through. Overshooting the connection budget for
// the length of one request is recoverable; a use-after-close on a pgx pool is
// a panic.
func (r *Registry) evictLocked() []*resident {
	var closing []*resident
	for r.recency.Len() > r.maxResident {
		oldest := r.recency.Back()
		if oldest == nil {
			break
		}
		orgID, _ := oldest.Value.(string)
		res, ok := r.residents[orgID]
		if !ok {
			r.recency.Remove(oldest)
			continue
		}
		r.recency.Remove(oldest)
		res.element = nil
		if res.refs > 0 {
			res.evicting = true
			continue
		}
		delete(r.residents, orgID)
		closing = append(closing, res)
	}
	return closing
}

// Close releases every resident store.
//
// Referenced residents are closed too: Close is process shutdown, where waiting
// for in-flight requests is the server's job and not the registry's. It does
// not close the shared base, which the registry does not own.
func (r *Registry) Close() error {
	r.mu.Lock()
	residents := make([]*resident, 0, len(r.residents))
	for _, res := range r.residents {
		residents = append(residents, res)
	}
	r.residents = make(map[string]*resident)
	r.recency.Init()
	r.mu.Unlock()

	closeAll(residents)
	return nil
}

// Resident reports how many tenant stores are currently open. Exposed for
// tests and for an operator metric: it is the number that must stay under the
// connection budget, so it is the number worth watching.
func (r *Registry) Resident() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.residents)
}

// closeAll closes a batch of evicted residents.
func closeAll(residents []*resident) {
	for _, res := range residents {
		closeServices(res.tenant, res.svc)
	}
}

// closeServices closes one tenant's services, logging rather than propagating a
// failure.
//
// Eviction happens on the path of a *different* tenant's request, so returning
// the error would fail a request that did nothing wrong and has no way to act
// on it. The leak is bounded — one tenant's pool — and observable, which is the
// better trade than a confusing failure.
func closeServices(t Tenant, svc *appbuild.Services) {
	if svc == nil {
		return
	}
	if err := svc.Close(); err != nil {
		slog.Warn("tenant: closing evicted tenant store failed",
			"org_id", t.OrgID, "schema", t.Schema, "error", err)
	}
}
