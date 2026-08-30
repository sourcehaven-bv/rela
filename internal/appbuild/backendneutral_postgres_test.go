//go:build postgres

package appbuild

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// This file must NOT import internal/store/pgstore.
//
// That is the whole test (TKT-L3FNEN AC-2). The capability interfaces used to
// name pgstore types in their signatures, so a second backend could satisfy
// them only by importing pgstore — decoupled discovery over a coupled contract.
// Every double below is built from store-package types alone; if a pgstore type
// crept back into a signature, this file would stop compiling rather than fail
// an assertion, which is the loudest possible failure.
//
// widen_assertions_postgres_test.go covers the same resolvers but DOES import
// pgstore (it asserts the real backend still satisfies them), so it cannot make
// this claim. The two are complementary.

// neutralSweeper implements store.VersionSweeper without any backend types.
type neutralSweeper struct {
	store.Store
	started bool
	gotCfg  store.SweepConfig
}

func (n *neutralSweeper) StartVersionSweep(_ store.ProjectionProvider, cfg store.SweepConfig) {
	n.started = true
	n.gotCfg = cfg
}

// neutralVersionProvider implements store.VersionServiceProvider.
type neutralVersionProvider struct {
	store.Store
	svc store.VersionService
}

func (n neutralVersionProvider) VersionStore() store.VersionService { return n.svc }

// neutralVersionService is a do-nothing store.VersionService. Its methods are
// never called — the resolver's contract is which handle it returns.
type neutralVersionService struct{ store.VersionService }

// neutralStateStore satisfies the wiring's rawStateStore structurally, the way
// a non-pgstore backend would: three methods, no state.KV import (a store may
// not depend on that application package).
type neutralStateStore struct{}

func (neutralStateStore) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (neutralStateStore) Put(context.Context, string, []byte) error   { return nil }
func (neutralStateStore) Delete(context.Context, string) error        { return nil }

func neutralBase(t *testing.T) store.Store {
	t.Helper()
	m := memstore.New()
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// TestCapabilitiesAreSatisfiableWithoutPgstore is AC-2: a backend outside
// pgstore can satisfy every capability.
func TestCapabilitiesAreSatisfiableWithoutPgstore(t *testing.T) {
	t.Run("version sweep", func(t *testing.T) {
		s := &neutralSweeper{Store: neutralBase(t)}
		startVersionSweepIfSupported(s, nil)
		if !s.started {
			t.Fatal("a store implementing store.VersionSweeper was not reached")
		}
	})

	t.Run("version service", func(t *testing.T) {
		want := neutralVersionService{}
		s := neutralVersionProvider{Store: neutralBase(t), svc: want}
		if got := versionServiceFor(s); got == nil {
			t.Fatal("a store implementing store.VersionServiceProvider was not reached")
		}
	})
}

// TestNeutralProviderReturningNilYieldsUntypedNil is the guard the ticket
// warns about: promoting the return types makes the typed-nil path reachable by
// MORE implementations, not fewer, so the nil checks became more load-bearing
// rather than redundant. A provider that hands back nothing must not produce a
// non-nil interface, which would pass every downstream nil-check and panic at
// write time.
func TestNeutralProviderReturningNilYieldsUntypedNil(t *testing.T) {
	s := neutralVersionProvider{Store: neutralBase(t), svc: nil}
	if got := versionServiceFor(s); got != nil {
		t.Errorf("versionServiceFor = %#v, want untyped nil", got)
	}
}

// TestSweepConfigCrossesPackagesUnchanged pins the alias decision. pgstore
// declares `SweepConfig = store.SweepConfig` (an alias, not a named type), so
// the two are interchangeable. A named type would compile here but make a value
// built in one package unusable in the other — the coupling this ticket removed,
// reintroduced in a form that greps clean.
func TestSweepConfigCrossesPackagesUnchanged(t *testing.T) {
	cfg := sweepConfigFromEnv()

	// Passing sweepConfigFromEnv()'s result straight into a parameter typed
	// store.SweepConfig is the assertion, and it is a COMPILE-time one: with a
	// named type instead of an alias this call would not build.
	//
	// (An explicit store.SweepConfig(cfg) conversion here is flagged as
	// unnecessary by the linter — which is itself the proof, since a conversion
	// between distinct named types would be required rather than redundant.)
	s := &neutralSweeper{Store: neutralBase(t)}
	s.StartVersionSweep(nil, cfg)
	if s.gotCfg != cfg {
		t.Errorf("config crossed the interface changed: got %+v, want %+v", s.gotCfg, cfg)
	}
}

// TestRawStateStoreIsSatisfiableWithoutPgstore covers the state half, which the
// ticket notes was left out of TKT-415WA7 entirely.
func TestRawStateStoreIsSatisfiableWithoutPgstore(t *testing.T) {
	var _ rawStateStore = neutralStateStore{}

	// And a store with no state capability must fall through to the FSKV, not
	// hand back a non-nil interface wrapping nothing.
	if got := stateKVFor(neutralBase(t)); got != nil {
		t.Errorf("stateKVFor = %#v, want untyped nil so the FSKV fallback engages", got)
	}
}
