package datamigration

import (
	"encoding/json"
	"testing"
	"time"
)

func writeLegacy(t *testing.T, kv *fakeKV, m legacyMarker) {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal legacy marker: %v", err)
	}
	if err := kv.Put(t.Context(), legacyMarkerKey, data); err != nil {
		t.Fatalf("write legacy marker: %v", err)
	}
}

func legacyFixture(t *testing.T, applied ...string) legacyMarker {
	t.Helper()
	proj := metaV1().ShapeProjection()
	raw, err := proj.JSON()
	if err != nil {
		t.Fatalf("projection JSON: %v", err)
	}
	return legacyMarker{
		ShapeHash:  proj.Hash(),
		Projection: raw,
		Applied:    applied,
		UpdatedAt:  time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
}

// The upgrade path. A store whose legacy marker says migrations have run must
// NOT read as un-bootstrapped — that would baseline it against the live schema
// and replay every one of them.
func TestLegacyBridge_AdoptsLegacyMarkerOnUpgrade(t *testing.T) {
	kv := newFakeKV()
	writeLegacy(t, kv, legacyFixture(t, "20260901120000-first.yaml"))

	b, err := NewLegacyBridge(newMigState(), kv)
	if err != nil {
		t.Fatalf("NewLegacyBridge: %v", err)
	}
	got, err := b.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("a legacy marker must be adopted, not read as un-bootstrapped")
	}
	if names := got.AppliedNames(); len(names) != 1 || names[0] != "20260901120000-first.yaml" {
		t.Errorf("applied = %v, want the legacy list carried across", names)
	}
	proj, err := got.ShapeProjection()
	if err != nil {
		t.Fatalf("ShapeProjection: %v", err)
	}
	if proj.Hash() != metaV1().ShapeProjection().Hash() {
		t.Error("the legacy projection did not carry across")
	}
}

// The new record wins once it exists: the legacy marker is a fallback, not a
// second source of truth.
func TestLegacyBridge_NewRecordWins(t *testing.T) {
	kv := newFakeKV()
	writeLegacy(t, kv, legacyFixture(t, "20260901120000-first.yaml"))

	next := newMigState()
	current, err := NewState(metaV2().ShapeProjection(), []AppliedEntry{
		{Name: "20260919143022-second.yaml"},
	}, time.Now())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if saveErr := next.Save(t.Context(), current); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	b, err := NewLegacyBridge(next, kv)
	if err != nil {
		t.Fatalf("NewLegacyBridge: %v", err)
	}
	got, err := b.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if names := got.AppliedNames(); len(names) != 1 || names[0] != "20260919143022-second.yaml" {
		t.Errorf("applied = %v, want the NEW record, not the legacy marker", names)
	}
}

// The rollback path. After a save, an older binary reading the legacy key must
// find a CURRENT marker — otherwise it sees an un-bootstrapped store and
// strands every pending migration.
func TestLegacyBridge_MirrorsWritesForRollback(t *testing.T) {
	kv := newFakeKV()
	b, err := NewLegacyBridge(newMigState(), kv)
	if err != nil {
		t.Fatalf("NewLegacyBridge: %v", err)
	}

	st, err := NewState(metaV2().ShapeProjection(), []AppliedEntry{
		{Name: "20260919143022-applied.yaml", AppliedAt: time.Now()},
	}, time.Now())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if saveErr := b.Save(t.Context(), st); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	raw, err := kv.Get(t.Context(), legacyMarkerKey)
	if err != nil {
		t.Fatalf("the legacy marker must be mirrored for rollback: %v", err)
	}
	var m legacyMarker
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("mirrored marker is not readable by the old format: %v", err)
	}
	if m.ShapeHash != metaV2().ShapeProjection().Hash() {
		t.Errorf("mirrored shape_hash = %s, want the current shape", m.ShapeHash)
	}
	if len(m.Applied) != 1 || m.Applied[0] != "20260919143022-applied.yaml" {
		t.Errorf("mirrored applied = %v, want the current list", m.Applied)
	}
}

// A legacy name that predates the timestamp scheme cannot be carried across,
// and the WHOLE marker must then be refused rather than partially adopted.
//
// Returning the surviving entries would be silent data corruption: under the
// old scheme every name is %04d-shaped, so every one fails and the adopted
// state would carry an empty applied list beside a valid projection. Resolve
// reads that as "nothing has run" and replans the entire chain against
// already-migrated content.
func TestLegacyBridge_RefusesUnconvertibleLegacyNames(t *testing.T) {
	t.Run("all names legacy", func(t *testing.T) {
		kv := newFakeKV()
		writeLegacy(t, kv, legacyFixture(t, "0001-rename.yaml", "0002-backfill.yaml"))

		b, err := NewLegacyBridge(newMigState(), kv)
		if err != nil {
			t.Fatalf("NewLegacyBridge: %v", err)
		}
		got, err := b.Load(t.Context())
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got != nil {
			t.Fatalf("an unconvertible legacy marker must be refused entirely, got applied=%v", got.AppliedNames())
		}
	})

	// The mixed case is the dangerous one: a partial adoption looks plausible.
	t.Run("mixed names", func(t *testing.T) {
		kv := newFakeKV()
		writeLegacy(t, kv, legacyFixture(t, "0001-old-style.yaml", "20260901120000-new-style.yaml"))

		b, err := NewLegacyBridge(newMigState(), kv)
		if err != nil {
			t.Fatalf("NewLegacyBridge: %v", err)
		}
		got, err := b.Load(t.Context())
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got != nil {
			t.Fatalf("a partially-convertible marker must be refused, not partially adopted: %v", got.AppliedNames())
		}
	})
}

// A corrupt legacy marker falls back to bootstrap rather than failing: this
// path is only reached when the NEW store has nothing, so the outcome is the
// same one an operator would get if the old marker had never existed.
func TestLegacyBridge_CorruptLegacyMarkerIsAbsent(t *testing.T) {
	kv := newFakeKV()
	if err := kv.Put(t.Context(), legacyMarkerKey, []byte("{not json")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	b, err := NewLegacyBridge(newMigState(), kv)
	if err != nil {
		t.Fatalf("NewLegacyBridge: %v", err)
	}
	got, err := b.Load(t.Context())
	if err != nil {
		t.Fatalf("a corrupt legacy marker must not fail the load: %v", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil (bootstrap)", got)
	}
}

// With no state.KV there is no legacy marker to reconcile, so the bridge must
// be a pass-through rather than a wrapper that swallows writes.
func TestNewLegacyBridge_PassesThroughWithoutKV(t *testing.T) {
	next := newMigState()
	got, err := NewLegacyBridge(next, nil)
	if err != nil {
		t.Fatalf("NewLegacyBridge: %v", err)
	}
	if got != StateStore(next) {
		t.Error("with no KV the bridge should return the underlying store unwrapped")
	}
}

func TestNewLegacyBridge_RejectsNilNext(t *testing.T) {
	if _, err := NewLegacyBridge(nil, newFakeKV()); err == nil {
		t.Error("a nil underlying store must be rejected")
	}
}
