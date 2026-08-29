package tenant_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// countingOpener hands out a distinct zero-valued Services per call and records
// how many were opened.
//
// A zero-valued *appbuild.Services is a legitimate stand-in here because
// Services.Close nil-checks every field it tears down, so it exercises the
// registry's lifecycle without needing a database. The real store's behavior
// is covered by the postgres isolation test, which is where it belongs — these
// tests are about the registry's bookkeeping, and a real store would make them
// database-gated for no extra coverage of the logic under test.
type countingOpener struct {
	mu     sync.Mutex
	opened []string
	fail   map[string]error
}

func (c *countingOpener) open(_ context.Context, t tenant.Tenant) (*appbuild.Services, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err, ok := c.fail[t.OrgID]; ok {
		return nil, err
	}
	c.opened = append(c.opened, t.OrgID)
	return &appbuild.Services{}, nil
}

func (c *countingOpener) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.opened)
}

func testResolver(t *testing.T, orgs ...string) tenant.Resolver {
	t.Helper()
	tenants := make([]tenant.Tenant, 0, len(orgs))
	for i, o := range orgs {
		tenants = append(tenants, tenant.Tenant{
			OrgID:  o,
			Schema: fmt.Sprintf("tenant_%d", i),
			DSN:    fmt.Sprintf("host=localhost search_path=tenant_%d", i),
		})
	}
	r, err := tenant.NewMapResolver(tenants)
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}
	return r
}

func TestRegistry_RequiresCollaborators(t *testing.T) {
	t.Parallel()

	opener := (&countingOpener{}).open
	if _, err := tenant.NewRegistry(nil, opener, 2); err == nil {
		t.Error("NewRegistry accepted a nil resolver")
	}
	if _, err := tenant.NewRegistry(testResolver(t, "a"), nil, 2); err == nil {
		t.Error("NewRegistry accepted a nil opener")
	}
}

// TestRegistry_UnknownTenantGetsNoStore is the registry's half of the
// fail-closed property: an org that does not resolve must never reach an
// opener, so it cannot connect to anything at all.
func TestRegistry_UnknownTenantGetsNoStore(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "org-a"), opener.open, 2)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	for _, orgID := range []string{"", "org-unknown"} {
		lease, err := reg.Acquire(t.Context(), orgID)
		if !errors.Is(err, tenant.ErrUnknownTenant) {
			t.Fatalf("Acquire(%q) error = %v, want ErrUnknownTenant", orgID, err)
		}
		if lease != nil {
			t.Fatalf("Acquire(%q) returned a lease alongside an error", orgID)
		}
	}
	if opener.count() != 0 {
		t.Errorf("opener ran %d times for unresolvable orgs; it must never be reached", opener.count())
	}
	if reg.Resident() != 0 {
		t.Errorf("Resident() = %d after only failed acquisitions, want 0", reg.Resident())
	}
}

// TestRegistry_OpenFailureYieldsNoLease pins that a store that fails to open is
// an error and not a half-usable lease — the same fail-closed rule one level
// down from resolution.
func TestRegistry_OpenFailureYieldsNoLease(t *testing.T) {
	t.Parallel()

	boom := errors.New("connection refused")
	opener := &countingOpener{fail: map[string]error{"org-a": boom}}
	reg, err := tenant.NewRegistry(testResolver(t, "org-a"), opener.open, 2)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	lease, err := reg.Acquire(t.Context(), "org-a")
	if !errors.Is(err, boom) {
		t.Fatalf("Acquire error = %v, want it to wrap %v", err, boom)
	}
	if lease != nil {
		t.Fatal("Acquire returned a lease for a store that failed to open")
	}
	if reg.Resident() != 0 {
		t.Errorf("Resident() = %d after a failed open, want 0", reg.Resident())
	}
}

// TestRegistry_NilServicesRefused covers the one route by which a nil store
// could enter the resident set and be handed to a request.
// nilOpener is the mistake the registry must catch: an opener that reports
// success while handing back nothing. Named rather than inlined so the
// nilnil exemption sits on the declaration and explains itself — returning
// (nil, nil) is the bug under test, not an accident.
func nilOpener(context.Context, tenant.Tenant) (*appbuild.Services, error) {
	return nil, nil //nolint:nilnil // the invalid return this test exists to reject
}

func TestRegistry_NilServicesRefused(t *testing.T) {
	t.Parallel()

	reg, err := tenant.NewRegistry(testResolver(t, "org-a"), nilOpener, 2)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	lease, err := reg.Acquire(t.Context(), "org-a")
	if err == nil {
		t.Fatal("an opener returning (nil, nil) was accepted")
	}
	if lease != nil {
		t.Fatal("a lease was returned over nil services")
	}
}

func TestRegistry_ReusesResidentStore(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "org-a"), opener.open, 2)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	first, err := reg.Acquire(t.Context(), "org-a")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	second, err := reg.Acquire(t.Context(), "org-a")
	if err != nil {
		t.Fatalf("Acquire (second): %v", err)
	}
	if first.Services() != second.Services() {
		t.Error("two acquisitions of one tenant returned different stores")
	}
	if opener.count() != 1 {
		t.Errorf("opened %d stores for one tenant, want 1", opener.count())
	}
	first.Release()
	second.Release()
}

// TestRegistry_BoundsResidentSet is the resident-set property RES-D54281's
// horizon requires: open stores are capped by an operator-set number, not by
// how many tenants exist. Each open store costs on the order of 17 database
// connections, so without this the tenant count would be bounded by
// max_connections.
func TestRegistry_BoundsResidentSet(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "a", "b", "c", "d", "e"), opener.open, 2)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	for _, org := range []string{"a", "b", "c", "d", "e"} {
		lease, err := reg.Acquire(t.Context(), org)
		if err != nil {
			t.Fatalf("Acquire(%s): %v", org, err)
		}
		// Released immediately, so nothing is pinned and the bound is exact.
		lease.Release()
		if got := reg.Resident(); got > 2 {
			t.Fatalf("Resident() = %d after acquiring %s, want <= 2", got, org)
		}
	}
	if got := reg.Resident(); got != 2 {
		t.Errorf("Resident() = %d after five tenants, want 2", got)
	}
	if opener.count() != 5 {
		t.Errorf("opened %d stores, want 5 (each evicted tenant reopens)", opener.count())
	}
}

// TestRegistry_EvictionLeavesSiblingsUsable pins the property that makes
// per-tenant eviction safe at all: closing one tenant's Services tears down
// only what that Services was assembled with. If eviction ever reached
// something shared, evicting one tenant would break every other.
func TestRegistry_EvictionLeavesSiblingsUsable(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "a", "b"), opener.open, 1)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	leaseA, err := reg.Acquire(t.Context(), "a")
	if err != nil {
		t.Fatalf("Acquire(a): %v", err)
	}
	svcA := leaseA.Services()
	leaseA.Release()

	// Acquiring b evicts a, since the bound is one.
	leaseB, err := reg.Acquire(t.Context(), "b")
	if err != nil {
		t.Fatalf("Acquire(b): %v", err)
	}
	defer leaseB.Release()

	if leaseB.Services() == svcA {
		t.Fatal("tenant b was handed tenant a's store")
	}
	if leaseB.Tenant.OrgID != "b" {
		t.Errorf("lease tenant = %q, want b", leaseB.Tenant.OrgID)
	}
	// b's store must still be usable after a was evicted and closed.
	if err := leaseB.Services().Close(); err != nil {
		t.Errorf("closing b after a was evicted: %v", err)
	}
}

// TestRegistry_EvictionWaitsForInFlightUse is the use-after-close guard. A
// referenced store must not be closed under a request: in a pgx pool that is a
// panic, not an error, so it would take the process down rather than fail one
// request.
func TestRegistry_EvictionWaitsForInFlightUse(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "a", "b"), opener.open, 1)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	// Hold a's lease: a request is in flight.
	leaseA, err := reg.Acquire(t.Context(), "a")
	if err != nil {
		t.Fatalf("Acquire(a): %v", err)
	}

	// Acquiring b pushes a out of the bound while a is still referenced.
	leaseB, err := reg.Acquire(t.Context(), "b")
	if err != nil {
		t.Fatalf("Acquire(b): %v", err)
	}
	defer leaseB.Release()

	// a is still resident because it is still in use. The bound is a bound on
	// idle stores; overshooting it for the length of one request is the
	// deliberate trade against a use-after-close.
	if reg.Resident() != 2 {
		t.Errorf("Resident() = %d while a is in flight, want 2 (a pending eviction)", reg.Resident())
	}
	if err := leaseA.Services().Close(); err != nil {
		t.Errorf("a's store was closed out from under an in-flight lease: %v", err)
	}

	// Releasing a completes its deferred eviction.
	leaseA.Release()
	if reg.Resident() != 1 {
		t.Errorf("Resident() = %d after releasing the evicted tenant, want 1", reg.Resident())
	}
}

// TestRegistry_DoubleReleaseIsSafe covers a handler that defers Release and also
// releases explicitly. A second release must not drop another caller's
// reference, which would re-open the use-after-close Lease exists to prevent.
func TestRegistry_DoubleReleaseIsSafe(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "a"), opener.open, 1)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	first, err := reg.Acquire(t.Context(), "a")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	second, err := reg.Acquire(t.Context(), "a")
	if err != nil {
		t.Fatalf("Acquire (second): %v", err)
	}

	first.Release()
	first.Release() // the accidental double release
	first.Release()

	// The second holder's reference must have survived, so the store is
	// still resident and still usable.
	if reg.Resident() != 1 {
		t.Fatalf("Resident() = %d after a double release, want 1", reg.Resident())
	}
	if err := second.Services().Close(); err != nil {
		t.Errorf("store closed by a double release: %v", err)
	}
	second.Release()
}

// TestRegistry_ConcurrentAcquireIsSafe runs under -race. Concurrent first
// requests for one tenant may open more than one store — wasteful, not
// incorrect — but exactly one must end up resident and every caller must get a
// usable lease.
func TestRegistry_ConcurrentAcquireIsSafe(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "a", "b", "c"), opener.open, 2)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	const goroutines = 24
	var wg sync.WaitGroup
	orgs := []string{"a", "b", "c"}
	errs := make(chan error, goroutines)
	for i := range goroutines {
		wg.Add(1)
		go func(org string) {
			defer wg.Done()
			lease, err := reg.Acquire(context.Background(), org)
			if err != nil {
				errs <- err
				return
			}
			if lease.Services() == nil {
				errs <- errors.New("lease carried nil services")
			}
			lease.Release()
		}(orgs[i%len(orgs)])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Acquire: %v", err)
	}
	if got := reg.Resident(); got > 2 {
		t.Errorf("Resident() = %d after concurrent traffic, want <= 2", got)
	}
}

func TestRegistry_CloseReleasesEverything(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	reg, err := tenant.NewRegistry(testResolver(t, "a", "b"), opener.open, 4)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	for _, org := range []string{"a", "b"} {
		lease, err := reg.Acquire(t.Context(), org)
		if err != nil {
			t.Fatalf("Acquire(%s): %v", org, err)
		}
		lease.Release()
	}
	if reg.Resident() != 2 {
		t.Fatalf("Resident() = %d, want 2", reg.Resident())
	}
	if err := reg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if reg.Resident() != 0 {
		t.Errorf("Resident() = %d after Close, want 0", reg.Resident())
	}
}

// TestRegistry_DefaultBoundApplied pins that a non-positive bound takes the
// documented default rather than meaning "unbounded" — an unbounded resident
// set would exhaust the cluster's connections under enough tenants.
func TestRegistry_DefaultBoundApplied(t *testing.T) {
	t.Parallel()

	opener := &countingOpener{}
	orgs := make([]string, tenant.DefaultMaxResident+3)
	for i := range orgs {
		orgs[i] = fmt.Sprintf("org-%d", i)
	}
	reg, err := tenant.NewRegistry(testResolver(t, orgs...), opener.open, 0)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	for _, org := range orgs {
		lease, err := reg.Acquire(t.Context(), org)
		if err != nil {
			t.Fatalf("Acquire(%s): %v", org, err)
		}
		lease.Release()
	}
	if got := reg.Resident(); got != tenant.DefaultMaxResident {
		t.Errorf("Resident() = %d, want DefaultMaxResident (%d)", got, tenant.DefaultMaxResident)
	}
}
