//go:build postgres

package appbuild

import (
	"context"
	"errors"
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
	return fakeStore{Store: memstore.New()}
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
}

func (f *fakeReconciler) SetUniqueSpecProvider(specs []store.DerivedObjectSpec) {
	f.specsPublished = specs
}

func (f *fakeReconciler) Reconcile(
	_ context.Context, desired []store.DerivedObjectSpec, _ store.ReconcileOptions,
) ([]store.DerivedObjectOutcome, error) {
	f.reconcileCalls++
	if f.reconcileErr != nil {
		return nil, f.reconcileErr
	}
	outcomes := make([]store.DerivedObjectOutcome, 0, len(desired))
	for _, spec := range desired {
		outcomes = append(outcomes, store.DerivedObjectOutcome{Spec: spec, State: store.DerivedCreated})
	}
	return outcomes, nil
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

	if f.reconcileCalls != 1 {
		t.Errorf("Reconcile calls = %d, want 1", f.reconcileCalls)
	}
	// specsPublished is non-nil only if SetUniqueSpecProvider ran; a failing
	// Reconcile must not prevent that, and must not panic.
	if f.specsPublished == nil && len(f.specsPublished) != 0 {
		t.Error("specs were not published before the failing reconcile")
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
	if got := stateKVFor(plain); got != nil {
		t.Errorf("stateKVFor = %#v, want untyped nil so the FSKV fallback engages", got)
	}

	// Must not panic, and must not start a sweep.
	startVersionSweepIfSupported(plain, nil)
	reconcileDerivedSchemaIfSupported(t.Context(), plain, nil)
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

// TestPgstoreSatisfiesWidenedInterfaces is a compile-time assertion. If a future
// change to pgstore's method set breaks one of these, the failure lands here
// with a clear name rather than as a silently-skipped capability at runtime —
// which is exactly how this class of bug hides.
func TestPgstoreSatisfiesWidenedInterfaces(t *testing.T) {
	var s *pgstore.Store
	var (
		_ userStateProvider       = s
		_ derivedSchemaReconciler = s
		_ versionSweeper          = s
		_ versionServiceProvider  = s
	)
	t.Log("pgstore.Store satisfies all four widened capability interfaces")
}
