package tenant

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
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

// Resident is what an [Opener] opens for one tenant and the [Registry] keeps
// while the tenant is resident: at least the tenant's store, and for a host
// whatever it built over that store. Close tears all of it down.
//
// The registry is generic over it so a host can keep its per-tenant
// application next to the store it reads, and close both together, without
// a second map beside the registry whose lifecycle could drift from it.
type Resident interface {
	Close() error
}

// Revoker is an optional [Resident] method the registry calls when a
// tenant stops being served while a request still holds it. It must end the
// resident's long-lived streams, so that their leases are released and the
// resident closes, which releases the tenant's schema lock. It must not
// close what a running request still reads.
type Revoker interface {
	Revoke()
}

// Opener opens the resident value for one tenant.
//
// Injected rather than built here, because the store a build opens is
// build-tag-dependent (PostgreSQL, filesystem, memory) while this package is
// not. [SharedBaseOpener] is the production opener of the store. The seam is
// also what the tests use to count opens and closes without a database.
//
// Implementations must return a usable value or an error, never both, and
// never a nil value without an error: the fail-closed rule from the package
// doc, applied one level down.
type Opener[R Resident] func(ctx context.Context, t Tenant) (R, error)

// Opened is one tenant opened by [SharedBaseOpener]: its services, and the
// connection that holds the tenant's schema lock shared while they are open.
// The lock is what makes [Erase] refuse a tenant a server has open.
type Opened struct {
	// Services are the tenant's services. Close the Opened, not these, so
	// the lock is released after them.
	Services *appbuild.Services

	pin       io.Closer
	closeOnce sync.Once
	closeErr  error
}

// Close closes the services, then releases the schema lock, so the schema
// cannot be dropped before the store over it has closed. Safe to call more
// than once; later calls return the first result.
func (o *Opened) Close() error {
	o.closeOnce.Do(func() {
		if o.Services != nil {
			o.closeErr = o.Services.Close()
		}
		if o.pin != nil {
			if perr := o.pin.Close(); perr != nil {
				slog.Warn("tenant: releasing the schema lock", "error", perr)
			}
		}
	})
	return o.closeErr
}

// SharedBaseOpener opens each tenant over one shared base. The base holds the
// parsed metamodel, `acl.yaml` and the rest of the operator config, so they are
// parsed once for every tenant. Each open adds only the tenant's own backend:
// a pool, store and search for the tenant's DSN.
//
// Before anything is opened, the opener takes the tenant's schema lock
// shared and checks that the connection lands in the tenant's own schema
// (see pinSchema). It holds the lock until the tenant closes.
//
// Each tenant also gets its own audit sink, a directory named after its
// schema under auditRoot (TKT-SNVR8UV, see [AuditRoot]). The schema is the
// name to use because it is derived by [SchemaFor], checked against a strict
// pattern and unique per tenant, while an org id is an arbitrary string from
// the identity provider. One sink for every tenant would interleave all their
// audit records, and [Erase] could then not remove one tenant's records
// without touching the others'.
//
// The build-tag split stays in appbuild: [appbuild.SharedBase.OpenAndAssemble]
// is where the build chooses its backend, and it refuses on a build whose store
// is not addressed by a DSN.
//
// Nil: rejected — an opener with no base would fail on the first request. An
// empty auditRoot is rejected for the same reason.
func SharedBaseOpener(base *appbuild.SharedBase, auditRoot string) (Opener[*Opened], error) {
	if base == nil {
		return nil, errors.New("tenant.SharedBaseOpener: base is required")
	}
	if auditRoot == "" {
		return nil, errors.New("tenant.SharedBaseOpener: audit root is required")
	}
	return func(ctx context.Context, t Tenant) (*Opened, error) {
		// Opening a tenant's store must outlive the request that triggered
		// it: a cancelled request must not leave a half-migrated schema
		// behind.
		ctx = context.WithoutCancel(ctx)
		pin, err := pinSchema(ctx, t.DSN, t.Schema)
		if err != nil {
			return nil, err
		}
		sink, err := audit.NewFilesystem(filepath.Join(auditRoot, t.Schema))
		if err != nil {
			_ = pin.Close()
			return nil, fmt.Errorf("audit sink: %w", err)
		}
		svc, err := base.OpenAndAssemble(ctx, t.DSN, sink)
		if err != nil {
			_ = pin.Close()
			return nil, err
		}
		return &Opened{Services: svc, pin: pin}, nil
	}, nil
}

// Lease is a borrowed reference to one tenant's resident value.
//
// It exists because eviction and use overlap. The resident set is bounded, so
// acquiring tenant F may evict tenant A — and if a request is still reading
// through A's store, closing it is a use-after-close, which in a pgx pool is a
// panic rather than an error. Counting references is the only way to know, and
// hoping request lifetimes are shorter than eviction pressure is not knowing.
//
// Every successful [Registry.Acquire] must be matched by exactly one
// [Lease.Release], conventionally deferred.
type Lease[R Resident] struct {
	// Tenant is the resolved tenant this lease serves, carried so a caller can
	// log or audit the tenant it acted as without a second lookup.
	Tenant Tenant

	// res is the resident this lease counts against. Release goes through
	// this pointer and not through a lookup by org id: the resident a lease
	// was taken on is the one it must release, even if the map entry for the
	// org has moved on since.
	res      *resident[R]
	registry *Registry[R]
	released bool
}

// Value returns the tenant's resident value this lease borrows.
//
// The returned value is valid only until [Lease.Release]. It is scoped to
// exactly one tenant: this is where the isolation guarantee is actually
// delivered, since every query issued through the tenant's store resolves
// through a `search_path` that names one tenant's schema and no other's.
func (l *Lease[R]) Value() R { return l.res.value }

// Release returns the lease. Safe to call more than once so a handler can
// defer it unconditionally; only the first call counts, because a double
// release would drop another caller's reference and re-open the
// use-after-close it exists to prevent.
func (l *Lease[R]) Release() {
	if l == nil || l.released {
		return
	}
	l.released = true
	l.registry.release(l.res)
}

// ErrAtCapacity reports that opening another tenant would exceed the
// registry's hard ceiling on open residents. It is a temporary refusal: the
// caller should answer "try again later" rather than serve the request some
// other way.
var ErrAtCapacity = errors.New("tenant: too many tenants open")

// ErrClosed reports that the registry was closed. Nothing is opened after
// Close, so a late request during shutdown cannot leave a resident behind
// that nothing will ever close.
var ErrClosed = errors.New("tenant: registry closed")

// ErrRevoking reports a request for an org whose previous resident was
// revoked and is still held by a request. The org is served again, but its
// old resident's streams were ended, so it is not reused; a second resident
// beside it would put two stores on one schema. It is a temporary refusal,
// like [ErrAtCapacity].
var ErrRevoking = errors.New("tenant: still closing the previous instance of this tenant")

// Option tunes a [Registry].
type Option func(*options)

type options struct {
	maxOpen int
}

// WithMaxOpen sets the hard ceiling on open residents: resident, pinned past
// the bound by a long request, being opened, or being closed. n <= 0 takes
// twice the resident bound.
//
// The resident bound alone does not cap connections. A tenant evicted while
// a live-update stream holds it stays open until the stream ends, so with
// enough streaming tenants the open count would follow the number of tenants
// with an open browser tab. Above this ceiling a new tenant is refused with
// [ErrAtCapacity] instead.
func WithMaxOpen(n int) Option {
	return func(o *options) { o.maxOpen = n }
}

// resident is one tenant's open value and its bookkeeping.
type resident[R Resident] struct {
	tenant   Tenant
	value    R
	refs     int
	element  *list.Element // position in the registry's recency list; nil while evicting
	evicting bool
	closed   bool
	// revoked is set when the resolver refused the org while a request
	// held the resident. It stays in the map until its last release, so
	// it counts against the ceiling and Close closes it, but no Acquire
	// takes it again.
	revoked bool
}

// pendingOpen is one tenant being opened. Later requests for the same org
// wait on done rather than open a second resident.
type pendingOpen struct {
	done chan struct{}
	err  error // set before done is closed
}

// Registry hands out per-tenant resident values over one shared configuration.
//
// It is the multi-store host `appbuild.SharedBase` was built for. With
// [SharedBaseOpener], every tenant is assembled over one base: one parsed
// metamodel and one parsed ACL policy for N stores. The tenants differ only in
// content, so the expensive half of construction happens once for all of them.
// The shared config is a property of the opener, not of the registry: an
// [Opener] that builds its own config per tenant parses it per tenant.
//
// # What it guarantees
//
//   - An org that does not resolve gets no store (see the package doc).
//   - At most MaxResident idle residents are open at once, independently of
//     how many tenants exist. That decoupling is the property that makes
//     tenant count an operations question instead of an architecture one.
//   - At most maxOpen residents exist at all, counting pinned, opening and
//     closing ones (see [WithMaxOpen]).
//   - At most one resident per org. Concurrent first requests share one
//     open, and an evicting resident still in use is revived by the next
//     Acquire for its org rather than shadowed by a second store for the
//     same tenant.
//   - Evicting one tenant closes that tenant's resident and nothing else.
//     For the store this is inherited rather than implemented:
//     Services.Close tears down only the store and search closer it was
//     assembled with, never anything owned by the shared base.
//
// # What it does not do
//
// It never creates a schema. An unknown org is denied, not provisioned
// (TKT-TNPRV8), and no tenant is ever erased here (TKT-TNERAS).
type Registry[R Resident] struct {
	resolver    Resolver
	open        Opener[R]
	maxResident int
	maxOpen     int

	mu        sync.Mutex
	residents map[string]*resident[R]
	pending   map[string]*pendingOpen
	recency   *list.List    // front = most recently used
	closing   int           // residents handed to a background close that has not finished
	closeDone chan struct{} // closed and replaced each time a background close finishes
	closed    bool
	closers   sync.WaitGroup
}

// NewRegistry builds a registry over a resolver and an opener.
//
// maxResident <= 0 takes [DefaultMaxResident]. Both collaborators are required
// and nil is rejected at construction rather than at first request, per
// CLAUDE.md — and here for a sharper reason than usual: a registry that failed
// only on use would fail inside a request, where the natural recovery is to
// serve the request some other way.
func NewRegistry[R Resident](resolver Resolver, open Opener[R], maxResident int, opts ...Option) (*Registry[R], error) {
	if resolver == nil {
		return nil, errors.New("tenant.NewRegistry: resolver is required")
	}
	if open == nil {
		return nil, errors.New("tenant.NewRegistry: opener is required")
	}
	if maxResident <= 0 {
		maxResident = DefaultMaxResident
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.maxOpen <= 0 {
		o.maxOpen = 2 * maxResident
	}
	if o.maxOpen < maxResident+1 {
		// One slot above the bound is what lets a new tenant open before the
		// idle one it pushes out has closed.
		o.maxOpen = maxResident + 1
	}
	return &Registry[R]{
		resolver:    resolver,
		open:        open,
		maxResident: maxResident,
		maxOpen:     o.maxOpen,
		residents:   make(map[string]*resident[R]),
		pending:     make(map[string]*pendingOpen),
		recency:     list.New(),
		closeDone:   make(chan struct{}),
	}, nil
}

// Acquire resolves orgID and leases its resident value, opening it if the
// tenant is not resident.
//
// Returns an error and a nil lease for any org it cannot resolve — including
// the empty org — so a caller cannot accidentally proceed with a usable handle.
// Check the error; do not check the lease for nil and carry on. An org that
// does not resolve opens nothing.
//
// The caller must Release the returned lease.
func (r *Registry[R]) Acquire(ctx context.Context, orgID string) (*Lease[R], error) {
	t, err := r.resolver.Resolve(ctx, orgID)
	if err != nil {
		if refused(err) {
			// The org is no longer served: suspended, being erased or
			// gone. Close what is still open for it.
			r.revoke(orgID)
		}
		return nil, err
	}

	for {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return nil, ErrClosed
		}
		if res, ok := r.residents[t.OrgID]; ok && res.revoked {
			r.mu.Unlock()
			return nil, fmt.Errorf("tenant %q: %w", t.OrgID, ErrRevoking)
		}
		if lease, ok := r.takeLocked(t.OrgID); ok {
			evicted := r.evictLocked()
			r.mu.Unlock()
			r.closeInBackground(evicted)
			return lease, nil
		}
		if p, ok := r.pending[t.OrgID]; ok {
			// Another request is opening this tenant. Share its result:
			// an open builds a whole pool and runs migrations, so N
			// parallel first requests must not open N of them.
			r.mu.Unlock()
			select {
			case <-p.done:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			if p.err != nil {
				return nil, p.err
			}
			continue
		}
		if len(r.residents)+len(r.pending)+r.closing >= r.maxOpen {
			// At the ceiling. An idle resident can make room: close it
			// and wait. Only a ceiling held entirely by residents in use
			// and opens in progress refuses.
			evicted := r.evictIdleLocked()
			if len(evicted) == 0 && r.closing == 0 {
				r.mu.Unlock()
				return nil, fmt.Errorf("tenant %q: %w", t.OrgID, ErrAtCapacity)
			}
			closed := r.closeDone
			r.mu.Unlock()
			r.closeInBackground(evicted)
			select {
			case <-closed:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}
		p := &pendingOpen{done: make(chan struct{})}
		r.pending[t.OrgID] = p
		r.mu.Unlock()
		return r.openPending(ctx, t, p)
	}
}

// openPending opens t for the pending entry p this goroutine registered,
// outside the lock: opening connects to a database and runs migrations, and
// holding the registry lock across that would stall every other tenant's
// requests behind one tenant's cold start.
func (r *Registry[R]) openPending(ctx context.Context, t Tenant, p *pendingOpen) (*Lease[R], error) {
	value, err := r.open(ctx, t)
	if err == nil && isNil(value) {
		// An opener returning (nil, nil) would put a nil store into the
		// resident set and hand it to a request. Refuse it here: this is
		// the zero-value-means-allowed shape the package doc warns about,
		// and the only place it could enter.
		err = errors.New("opener returned no services")
	}
	if err != nil {
		err = fmt.Errorf("tenant %q: open store: %w", t.OrgID, err)
	} else if rerr := r.recheck(ctx, t.OrgID); rerr != nil {
		closeValue(t, value)
		err = rerr
	}

	r.mu.Lock()
	delete(r.pending, t.OrgID)
	if err == nil && r.closed {
		err = ErrClosed
		defer closeValue(t, value)
	}
	if err != nil {
		p.err = err
		close(p.done)
		r.mu.Unlock()
		return nil, err
	}
	res := &resident[R]{tenant: t, value: value, refs: 1}
	res.element = r.recency.PushFront(t.OrgID)
	r.residents[t.OrgID] = res
	evicted := r.evictLocked()
	close(p.done)
	r.mu.Unlock()

	r.closeInBackground(evicted)
	return &Lease[R]{Tenant: t, res: res, registry: r}, nil
}

// recheck resolves orgID again once its open holds the schema lock. The
// status was read before the lock was taken, and a suspension or erasure
// that landed in between must not leave the tenant open. It runs without the
// request's cancellation: other requests may be waiting on this open, and a
// cancelled first request must not throw the opened tenant away.
func (r *Registry[R]) recheck(ctx context.Context, orgID string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recheckTimeout)
	defer cancel()
	_, err := r.resolver.Resolve(ctx, orgID)
	return err
}

// recheckTimeout bounds the re-check after a tenant opens.
const recheckTimeout = 10 * time.Second

// takeLocked leases the resident for orgID when there is one, reviving it if
// it was marked evicting. Requires r.mu.
//
// Reviving rather than opening a second store is what keeps "one resident per
// org" true: an evicting resident is still open (a request holds it), so
// opening another would put two pools on one schema and let the old one's
// release land on the new one's count.
func (r *Registry[R]) takeLocked(orgID string) (*Lease[R], bool) {
	res, ok := r.residents[orgID]
	if !ok {
		return nil, false
	}
	if res.evicting {
		res.evicting = false
		res.element = r.recency.PushFront(orgID)
	} else {
		r.recency.MoveToFront(res.element)
	}
	res.refs++
	return &Lease[R]{Tenant: res.tenant, res: res, registry: r}, true
}

// release drops one reference, closing the resident if it was already evicted
// and this was the last holder.
func (r *Registry[R]) release(res *resident[R]) {
	r.mu.Lock()
	res.refs--
	var closing []*resident[R]
	if res.refs <= 0 && res.evicting && !res.closed {
		if r.residents[res.tenant.OrgID] == res {
			delete(r.residents, res.tenant.OrgID)
		}
		closing = r.scheduleCloseLocked(closing, res)
	}
	r.mu.Unlock()

	r.closeInBackground(closing)
}

// evictLocked trims the resident set to the bound, returning the residents
// whose values the caller must close. Requires r.mu.
//
// Only unreferenced residents are closed here. One still in use is marked
// evicting instead: it leaves the recency list and the bound immediately, and
// its last [Lease.Release] closes it. So the bound is a bound on *idle* stores,
// and a burst of concurrent tenants can exceed it briefly rather than closing a
// store a request is reading through. Overshooting the connection budget for
// the length of one request is recoverable; a use-after-close on a pgx pool is
// a panic.
//
// A long-lived request (a live-update stream) therefore keeps its tenant
// open past the bound for as long as it runs. Ending the stream to free the
// slot would make the client reconnect, which acquires the tenant again and
// evicts another one, and under more streaming tenants than the bound that
// never settles. The hard ceiling (see [WithMaxOpen]) is what bounds this.
func (r *Registry[R]) evictLocked() []*resident[R] {
	var closing []*resident[R]
	for r.recency.Len() > r.maxResident {
		oldest := r.recency.Back()
		if oldest == nil {
			break
		}
		r.recency.Remove(oldest)
		orgID, _ := oldest.Value.(string)
		res, ok := r.residents[orgID]
		if !ok {
			continue
		}
		res.element = nil
		if res.refs > 0 {
			res.evicting = true
			continue
		}
		delete(r.residents, orgID)
		closing = r.scheduleCloseLocked(closing, res)
	}
	return closing
}

// evictIdleLocked evicts the least recently used resident no request holds,
// if there is one. Requires r.mu.
func (r *Registry[R]) evictIdleLocked() []*resident[R] {
	for e := r.recency.Back(); e != nil; e = e.Prev() {
		orgID, _ := e.Value.(string)
		res, ok := r.residents[orgID]
		if !ok || res.refs > 0 {
			continue
		}
		r.recency.Remove(e)
		res.element = nil
		delete(r.residents, orgID)
		return r.scheduleCloseLocked(nil, res)
	}
	return nil
}

// scheduleCloseLocked marks res closed and counts it as closing until
// closeInBackground has closed it. Requires r.mu.
func (r *Registry[R]) scheduleCloseLocked(closing []*resident[R], res *resident[R]) []*resident[R] {
	res.closed = true
	r.closing++
	r.closers.Add(1)
	return append(closing, res)
}

// closeInBackground closes residents scheduled by
// scheduleCloseLocked on a goroutine of its own.
//
// Not inline, because the caller is a request for a different tenant (the
// one whose open pushed these out) or the deferred release of a finished
// request. A resident's close stops its workers and drains its job queue and
// co-editing connections, which can take seconds no request should wait.
// [Registry.Close] waits for these goroutines.
func (r *Registry[R]) closeInBackground(residents []*resident[R]) {
	for _, res := range residents {
		go func() {
			defer r.closers.Done()
			closeValue(res.tenant, res.value)
			r.mu.Lock()
			r.closing--
			close(r.closeDone)
			r.closeDone = make(chan struct{})
			r.mu.Unlock()
		}()
	}
}

// Close releases every resident value and refuses later acquires with
// [ErrClosed].
//
// Referenced residents are closed too: Close is process shutdown, where waiting
// for in-flight requests is the server's job and not the registry's. Close
// waits for opens in progress and background closes to finish. It does not
// close the shared base, which the registry does not own.
func (r *Registry[R]) Close() error {
	r.mu.Lock()
	r.closed = true
	residents := make([]*resident[R], 0, len(r.residents))
	for _, res := range r.residents {
		if !res.closed {
			res.closed = true
			residents = append(residents, res)
		}
	}
	pending := make([]*pendingOpen, 0, len(r.pending))
	for _, p := range r.pending {
		pending = append(pending, p)
	}
	r.residents = make(map[string]*resident[R])
	r.recency.Init()
	r.mu.Unlock()

	for _, res := range residents {
		closeValue(res.tenant, res.value)
	}
	// An open in progress sees closed when it finishes and closes what it
	// opened.
	for _, p := range pending {
		<-p.done
	}
	r.closers.Wait()
	return nil
}

// refused reports whether err is a resolver's refusal of an org, as opposed
// to a lookup that failed. Only a refusal revokes a resident: a database
// that stopped answering must not close every tenant.
func refused(err error) bool {
	return errors.Is(err, ErrUnknownTenant) || errors.Is(err, ErrSchemaCollision)
}

// revoke takes orgID's resident out of service. An idle resident is closed
// now. One a request still holds is marked revoked, closed by its last
// release, and told to end its streams through [Revoker] so that release
// comes soon. A revoked resident is never taken again: its streams were
// ended.
func (r *Registry[R]) revoke(orgID string) {
	r.mu.Lock()
	res, ok := r.residents[orgID]
	if !ok || res.closed || res.revoked {
		r.mu.Unlock()
		return
	}
	if res.element != nil {
		r.recency.Remove(res.element)
		res.element = nil
	}
	var closing []*resident[R]
	inUse := res.refs > 0
	if inUse {
		res.evicting = true
		res.revoked = true
	} else {
		delete(r.residents, orgID)
		closing = r.scheduleCloseLocked(nil, res)
	}
	r.mu.Unlock()

	slog.Info("tenant: closing a tenant that is no longer served", "org_id", orgID, "schema", res.tenant.Schema)
	if inUse {
		if rv, ok := any(res.value).(Revoker); ok {
			rv.Revoke()
		}
	}
	r.closeInBackground(closing)
}

// Revalidate resolves every resident tenant again and revokes those the
// resolver now refuses. A host runs it on a timer, so that a suspended or
// erasing tenant that gets no more requests still closes and releases its
// schema lock, which lets an erasure proceed. A failed lookup revokes
// nothing.
func (r *Registry[R]) Revalidate(ctx context.Context) {
	r.mu.Lock()
	orgs := make([]string, 0, len(r.residents))
	for org := range r.residents {
		orgs = append(orgs, org)
	}
	r.mu.Unlock()
	for _, org := range orgs {
		if ctx.Err() != nil {
			return
		}
		if _, err := r.resolver.Resolve(ctx, org); err != nil && refused(err) {
			r.revoke(org)
		}
	}
}

// Resident reports how many tenants are resident, counting those pinned past
// the bound by a request and not those still closing in the background.
// Exposed for tests and for an operator metric.
func (r *Registry[R]) Resident() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.residents)
}

// closeValue closes one tenant's resident value, logging rather than
// propagating a failure.
//
// Eviction happens on the path of a *different* tenant's request, so returning
// the error would fail a request that did nothing wrong and has no way to act
// on it. The leak is bounded — one tenant's pool — and observable, which is the
// better trade than a confusing failure.
func closeValue[R Resident](t Tenant, value R) {
	if isNil(value) {
		return
	}
	if err := value.Close(); err != nil {
		slog.Warn("tenant: closing evicted tenant store failed",
			"org_id", t.OrgID, "schema", t.Schema, "error", err)
	}
}

// isNil reports whether v is nil, including a typed nil pointer inside the
// interface, which a plain comparison with nil does not catch.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
