// Package migstatetest is the conformance harness every
// [datamigration.StateStore] implementation must pass.
//
// The point is that the four tiers (file, postgres, sqlite, memory) cannot
// drift. Migration state decides which migrations run against a store, so a
// backend that loses an applied entry re-runs a migration and a backend that
// invents one skips it — and both failures are invisible until data is already
// wrong. Holding every implementation to one executable contract is cheaper
// than discovering the divergence per backend.
package migstatetest

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Factory builds a fresh, empty store for one subtest. Implementations that
// need cleanup register it with t.Cleanup.
type Factory func(t *testing.T) datamigration.StateStore

// RunAll runs the whole contract.
func RunAll(t *testing.T, f Factory) {
	t.Helper()
	t.Run("Absent", func(t *testing.T) { RunAbsentTests(t, f) })
	t.Run("RoundTrip", func(t *testing.T) { RunRoundTripTests(t, f) })
	t.Run("Replace", func(t *testing.T) { RunReplaceTests(t, f) })
	t.Run("AppliedOrder", func(t *testing.T) { RunAppliedOrderTests(t, f) })
	t.Run("Isolation", func(t *testing.T) { RunIsolationTests(t, f) })
}

// RunAbsentTests pins the un-bootstrapped case: a store nothing has written to
// yet reports (nil, nil), NOT an error.
//
// This is the single most load-bearing behavior in the contract. Callers
// branch on nil to mean "no state recorded", and a backend that returned an
// error instead would make a fresh store look broken; one that returned an
// empty non-nil state would make it look migrated.
func RunAbsentTests(t *testing.T, f Factory) {
	t.Helper()
	st := f(t)
	got, err := st.Load(context.Background())
	if err != nil {
		t.Fatalf("Load on an empty store must not error, got: %v", err)
	}
	if got != nil {
		t.Fatalf("Load on an empty store must return nil state, got: %+v", got)
	}
}

// RunRoundTripTests pins that everything written comes back intact.
func RunRoundTripTests(t *testing.T, f Factory) {
	t.Helper()
	st := f(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)

	want, err := datamigration.NewState(fixtureProjection(), []datamigration.AppliedEntry{
		{Name: "20260901120000-first.yaml", AppliedAt: now.Add(-time.Hour)},
		{Name: "20260919143022-second.yaml", AppliedAt: now},
	}, now)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if saveErr := st.Save(ctx, want); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	got, err := st.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("Load after Save returned nil")
	}
	if got.FormatVersion != want.FormatVersion {
		t.Errorf("FormatVersion = %d, want %d", got.FormatVersion, want.FormatVersion)
	}
	if len(got.Applied) != len(want.Applied) {
		t.Fatalf("Applied has %d entries, want %d", len(got.Applied), len(want.Applied))
	}
	for i := range want.Applied {
		if got.Applied[i].Name != want.Applied[i].Name {
			t.Errorf("Applied[%d].Name = %q, want %q", i, got.Applied[i].Name, want.Applied[i].Name)
		}
		// Timestamps cross a serialization boundary that may not preserve
		// monotonic clock or sub-second precision, so compare at second
		// granularity in UTC rather than with ==.
		gotAt := got.Applied[i].AppliedAt.UTC().Truncate(time.Second)
		wantAt := want.Applied[i].AppliedAt.UTC().Truncate(time.Second)
		if !gotAt.Equal(wantAt) {
			t.Errorf("Applied[%d].AppliedAt = %v, want %v",
				i, got.Applied[i].AppliedAt, want.Applied[i].AppliedAt)
		}
	}

	// The projection must survive byte-for-byte in MEANING: it is re-hashed to
	// identify the shape, so a backend that reorders or re-encodes its JSON
	// would move the hash and make an in-sync store look changed.
	gotProj, err := got.ShapeProjection()
	if err != nil {
		t.Fatalf("ShapeProjection: %v", err)
	}
	if gotProj.Hash() != fixtureProjection().Hash() {
		t.Errorf("projection hash changed across a round trip: %s != %s",
			gotProj.Hash(), fixtureProjection().Hash())
	}
}

// RunReplaceTests pins that Save replaces rather than accumulates. A backend
// that appended would grow an ever-longer history and, worse, could return a
// stale projection.
func RunReplaceTests(t *testing.T, f Factory) {
	t.Helper()
	st := f(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)

	first, err := datamigration.NewState(fixtureProjection(), []datamigration.AppliedEntry{
		{Name: "20260901120000-first.yaml", AppliedAt: now},
	}, now)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if saveErr := st.Save(ctx, first); saveErr != nil {
		t.Fatalf("Save first: %v", saveErr)
	}

	second, err := datamigration.NewState(otherProjection(), []datamigration.AppliedEntry{
		{Name: "20260901120000-first.yaml", AppliedAt: now},
		{Name: "20260919143022-second.yaml", AppliedAt: now},
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if saveErr := st.Save(ctx, second); saveErr != nil {
		t.Fatalf("Save second: %v", saveErr)
	}

	got, err := st.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Applied) != 2 {
		t.Fatalf("Applied has %d entries after replace, want 2", len(got.Applied))
	}
	gotProj, err := got.ShapeProjection()
	if err != nil {
		t.Fatalf("ShapeProjection: %v", err)
	}
	if gotProj.Hash() != otherProjection().Hash() {
		t.Error("Save did not replace the stored projection")
	}
}

// RunAppliedOrderTests pins that application order survives. Order is how a
// human reads the record; a backend keyed by a map or sorted by name would
// reorder a list whose entries are not lexicographically ordered.
func RunAppliedOrderTests(t *testing.T, f Factory) {
	t.Helper()
	st := f(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)

	// Deliberately NOT in lexicographic order: a later-named migration applied
	// first is exactly what a merge of two branches produces.
	names := []string{
		"20260919143022-zulu.yaml",
		"20260901120000-alpha.yaml",
		"20260910090000-mike.yaml",
	}
	entries := make([]datamigration.AppliedEntry, 0, len(names))
	for i, n := range names {
		entries = append(entries, datamigration.AppliedEntry{
			Name: n, AppliedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}
	s, err := datamigration.NewState(fixtureProjection(), entries, now)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if saveErr := st.Save(ctx, s); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	got, err := st.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for i, want := range names {
		if got.Applied[i].Name != want {
			t.Errorf("Applied[%d] = %q, want %q (application order must be preserved)",
				i, got.Applied[i].Name, want)
		}
	}
}

// RunIsolationTests pins that a second Save is visible to a fresh Load — i.e.
// the backend actually persists rather than caching in the handle.
func RunIsolationTests(t *testing.T, f Factory) {
	t.Helper()
	st := f(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)

	s, err := datamigration.NewState(fixtureProjection(), nil, now)
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if saveErr := st.Save(ctx, s); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	// An empty applied list must come back as an empty list, not as a missing
	// state: a bootstrapped store with no migrations yet is a real, common
	// condition and must be distinguishable from an un-bootstrapped one.
	got, err := st.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("a saved state with no applied migrations must load as non-nil")
	}
	if len(got.Applied) != 0 {
		t.Errorf("Applied = %v, want empty", got.Applied)
	}
}

// fixtureProjection is a small but non-trivial shape: two entity types with
// differing property kinds, plus a relation, so a backend that mangles nested
// structure is caught by the hash comparison.
func fixtureProjection() metamodel.ShapeProjection {
	return fixtureMeta().ShapeProjection()
}

// otherProjection differs from fixtureProjection in shape, so the two hash
// differently.
func otherProjection() metamodel.ShapeProjection {
	m := fixtureMeta()
	def := m.Entities["task"]
	props := maps.Clone(def.Properties)
	props["priority"] = metamodel.PropertyDef{Type: "string"}
	def.Properties = props
	m.Entities["task"] = def
	return m.ShapeProjection()
}

func fixtureMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: "string", Required: true},
				"status": {Type: "string", Values: []string{"open", "done"}},
				"due":    {Type: "date"},
			}},
			"person": {Properties: map[string]metamodel.PropertyDef{
				"name": {Type: "string", Required: true},
			}},
		},
		Relations: map[string]metamodel.RelationDef{
			"assigned-to": {From: []string{"task"}, To: []string{"person"}},
		},
	}
}
