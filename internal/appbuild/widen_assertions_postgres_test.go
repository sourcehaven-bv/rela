//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// These tests pin TKT-415WA7: appbuild discovers optional store capabilities by
// INTERFACE, not by asserting the concrete *pgstore.Store. Without them the
// refactor is unverified — the resolvers would still compile if someone
// reintroduced a concrete assertion, and the block would silently return.
//
// The fakes are deliberately NOT pgstore types. That is the whole point: if a
// resolver only accepted *pgstore.Store, every "fake is wired" test below would
// fail.

// --- fakes ---------------------------------------------------------------

// fakeStore embeds a real memstore so it satisfies store.Store without
// hand-writing 26 methods; the capability methods are added by the wrappers
// below. Embedding a genuine implementation also keeps the fakes honest — they
// cannot accidentally diverge from the interface.
type fakeStore struct{ store.Store }

func newFakeStore(t *testing.T) fakeStore {
	t.Helper()
	m := memstore.New()
	t.Cleanup(func() { _ = m.Close() })
	return fakeStore{Store: m}
}

type fakeUserStateStore struct {
	fakeStore
	called bool
	err    error
}

func (f *fakeUserStateStore) UserState() (userstate.Store, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	// A nil userstate.Store with a nil error is enough: the resolver's contract
	// is about which handle it returns, not what the handle does.
	return nil, nil //nolint:nilnil // exercising the resolver, not the backend
}

type fakeReconciler struct {
	fakeStore
	specsPublished []store.DerivedObjectSpec
	reconcileCalls int
	reconcileErr   error

	// calls records the method order. A bool pair cannot express "published
	// BEFORE reconciling", and the published slice is legitimately empty for a
	// metamodel with no unique properties — so its nilness proves nothing.
	calls []string
}

func (f *fakeReconciler) SetUniqueSpecProvider(specs []store.DerivedObjectSpec) {
	f.specsPublished = specs
	f.calls = append(f.calls, "SetUniqueSpecProvider")
}

func (f *fakeReconciler) Reconcile(
	_ context.Context, desired []store.DerivedObjectSpec, _ store.ReconcileOptions,
) ([]store.DerivedObjectOutcome, error) {
	f.reconcileCalls++
	f.calls = append(f.calls, "Reconcile")
	if f.reconcileErr != nil {
		return nil, f.reconcileErr
	}
	outcomes := make([]store.DerivedObjectOutcome, 0, len(desired))
	for _, spec := range desired {
		outcomes = append(outcomes, store.DerivedObjectOutcome{Spec: spec, State: store.DerivedCreated})
	}
	return outcomes, nil
}

// fakeNilVersionStore has the capability but hands back a nil pointer — the
// partial-init path a real backend could plausibly take. Boxing that into
// store.VersionService is what produces a non-nil interface wrapping nil.
type fakeNilVersionStore struct{ fakeStore }

func (fakeNilVersionStore) VersionStore() *pgstore.VersionStore { return nil }

// fakeNilUserState returns (nil, nil): no handle, no error.
type fakeNilUserState struct{ fakeStore }

func (fakeNilUserState) UserState() (userstate.Store, error) {
	return nil, nil //nolint:nilnil // deliberately exercising the (nil, nil) contract hole
}

type fakeSweeper struct {
	fakeStore
	started bool
	gotCfg  pgstore.SweepConfig
}

func (f *fakeSweeper) StartVersionSweep(_ pgstore.ProjectionProvider, cfg pgstore.SweepConfig) {
	f.started = true
	f.gotCfg = cfg
}

// --- AC-3: a non-pgstore store satisfying the interfaces is wired identically

func TestCapabilitiesDiscoveredByInterface_NotConcreteType(t *testing.T) {
	// A nil metamodel is the simplest valid input here: uniqueSpecsFromMetamodel
	// returns no specs for it, which exercises the discovery path (the thing
	// under test) without dragging a schema fixture into a wiring test.
	var meta *metamodel.Metamodel

	t.Run("user state", func(t *testing.T) {
		f := &fakeUserStateStore{fakeStore: newFakeStore(t)}
		storeUserStateFor(f)
		if !f.called {
			t.Fatal("storeUserStateFor did not reach a non-pgstore store implementing UserState")
		}
	})

	t.Run("derived schema reconciler", func(t *testing.T) {
		f := &fakeReconciler{fakeStore: newFakeStore(t)}
		reconcileDerivedSchemaIfSupported(t.Context(), f, meta)
		if f.reconcileCalls != 1 {
			t.Fatalf("Reconcile called %d times, want 1", f.reconcileCalls)
		}
	})

	t.Run("version sweep", func(t *testing.T) {
		f := &fakeSweeper{fakeStore: newFakeStore(t)}
		startVersionSweepIfSupported(f, meta)
		if !f.started {
			t.Fatal("startVersionSweepIfSupported did not reach a non-pgstore store implementing StartVersionSweep")
		}
	})
}

// TestDerivedSchemaPublishesSpecsBeforeReconciling pins the ordering the
// resolver's own comment calls out: specs are published FIRST so a violation of
// an already-present index still maps to a property even when the reconcile
// below degrades.
func TestDerivedSchemaPublishesSpecsBeforeReconciling(t *testing.T) {
	f := &fakeReconciler{
		fakeStore:    newFakeStore(t),
		reconcileErr: errors.New("reconcile unavailable"),
	}
	reconcileDerivedSchemaIfSupported(t.Context(), f, nil)

	// Order is the assertion. An earlier draft tested
	// `specsPublished == nil && len(specsPublished) != 0`, which is
	// tautologically false for a slice (a nil slice has len 0) and so could
	// never fail — a dead check masquerading as coverage.
	want := []string{"SetUniqueSpecProvider", "Reconcile"}
	if !slices.Equal(f.calls, want) {
		t.Errorf("call order = %v, want %v (specs must be published before "+
			"reconciling, so a violation of an existing index still maps to a "+
			"property when reconcile degrades)", f.calls, want)
	}
}

// --- AC-4: the genuinely-nil contract -------------------------------------

// TestResolversReturnUntypedNilWithoutCapability is the load-bearing negative
// test. A TYPED nil (e.g. `var s *pgstore.Store; return s`) satisfies the
// interface but compares != nil, which would silently defeat every downstream
// fallback — the failure mode three separate doc comments in this package warn
// about. Asserting on the interface value is the only way to catch it.
func TestResolversReturnUntypedNilWithoutCapability(t *testing.T) {
	plain := newFakeStore(t) // implements store.Store and nothing else

	if got := storeUserStateFor(plain); got != nil {
		t.Errorf("storeUserStateFor = %#v, want untyped nil so the KV fallback engages", got)
	}
	if got := versionServiceFor(plain); got != nil {
		t.Errorf("versionServiceFor = %#v, want untyped nil so version recording is skipped", got)
	}
	// stateKVFor is included for its nil contract only — NOT as evidence of
	// interface parity. It still discovers via pgstore.StateStoreFor's internal
	// concrete assertion (see its doc comment and TKT-L3FNEN).
	if got := stateKVFor(plain); got != nil {
		t.Errorf("stateKVFor = %#v, want untyped nil so the FSKV fallback engages", got)
	}

	// Must not panic, and must not start a sweep.
	startVersionSweepIfSupported(plain, nil)
	reconcileDerivedSchemaIfSupported(t.Context(), plain, nil)
}

// TestCapabilityPresentButHandleNilYieldsUntypedNil is the branch the
// no-capability test CANNOT reach, and the one that actually regressed when
// discovery widened from a concrete type to an interface.
//
// Asserting st.(*pgstore.Store) bounded the reachable implementations to one
// whose VersionStore() is unconditionally non-nil. An interface admits any
// implementation — so a nil pointer boxed into store.VersionService yields a
// NON-nil interface, and every downstream nil-check silently passes before
// panicking at write time.
func TestCapabilityPresentButHandleNilYieldsUntypedNil(t *testing.T) {
	t.Run("version service", func(t *testing.T) {
		f := fakeNilVersionStore{fakeStore: newFakeStore(t)}
		if got := versionServiceFor(f); got != nil {
			t.Errorf("versionServiceFor = %#v, want untyped nil; a typed nil "+
				"passes downstream nil-checks and panics on first use", got)
		}
	})

	t.Run("user state", func(t *testing.T) {
		f := fakeNilUserState{fakeStore: newFakeStore(t)}
		if got := storeUserStateFor(f); got != nil {
			t.Errorf("storeUserStateFor = %#v, want untyped nil", got)
		}
	})
}

// TestUserStateErrorYieldsUntypedNil covers the other typed-nil path: the
// capability is present but fails. The resolver must still hand back an untyped
// nil rather than an interface wrapping a broken handle.
func TestUserStateErrorYieldsUntypedNil(t *testing.T) {
	f := &fakeUserStateStore{fakeStore: newFakeStore(t), err: errors.New("pool exhausted")}
	if got := storeUserStateFor(f); got != nil {
		t.Errorf("storeUserStateFor = %#v on error, want untyped nil", got)
	}
}

// --- AC-2: pgstore still satisfies every widened interface ----------------

// Compile-time assertions, at file scope rather than wrapped in a func Test:
// they cannot fail at runtime, so a test would only add a fake green line to
// the report. If a change to pgstore's method set breaks one, the build fails
// here — instead of the capability being silently skipped at wiring time,
// which is exactly how this class of bug hides.
var (
	_ userStateProvider       = (*pgstore.Store)(nil)
	_ derivedSchemaReconciler = (*pgstore.Store)(nil)
	_ versionSweeper          = (*pgstore.Store)(nil)
	_ versionServiceProvider  = (*pgstore.Store)(nil)
)
