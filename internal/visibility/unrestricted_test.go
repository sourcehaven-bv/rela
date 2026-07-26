package visibility_test

import (
	"context"
	"iter"
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TKT-1WV50C: visibility.Unrestricted NAMES the ungated read path so
// `grep -rn visibility.Unrestricted` enumerates every bypass. These tests
// pin the two properties that make the name trustworthy: it really is a
// pass-through (so converting a site cannot change behavior), and it is
// really not a store (so it cannot silently widen back).

func seedStore(t *testing.T) store.Store {
	t.Helper()
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "One"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Two"}},
		{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "Feat"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, "TKT-1", "implements", "FEAT-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	return st
}

// TestUnrestricted_IsPassThrough pins that wrapping changes nothing an
// ungated caller could observe. If this drifts, converting a wiring site
// to Unrestricted would be a behavior change rather than a rename.
func TestUnrestricted_IsPassThrough(t *testing.T) {
	st := seedStore(t)
	r := visibility.Unrestricted(st)
	ctx := context.Background()

	t.Run("GetEntity matches the store", func(t *testing.T) {
		got, err := r.GetEntity(ctx, "TKT-1")
		if err != nil {
			t.Fatalf("GetEntity: %v", err)
		}
		want, err := st.GetEntity(ctx, "TKT-1")
		if err != nil {
			t.Fatalf("store.GetEntity: %v", err)
		}
		// Value equality, not pointer identity: memstore hands out a
		// defensive copy per call, so even the store does not return the
		// same pointer twice. What must hold is that NOTHING is dropped —
		// no redaction, no filtering.
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unrestricted altered the entity (redaction or filtering "+
				"on an UNGATED reader):\n got %+v\nwant %+v", got, want)
		}
	})

	t.Run("GetEntity propagates a miss", func(t *testing.T) {
		if _, err := r.GetEntity(ctx, "NOPE-1"); err == nil {
			t.Error("expected an error for a missing entity")
		}
	})

	t.Run("ListEntities yields every entity", func(t *testing.T) {
		var ids []string
		for e, err := range r.ListEntities(ctx, store.EntityQuery{}) {
			if err != nil {
				t.Fatalf("ListEntities: %v", err)
			}
			ids = append(ids, e.ID)
		}
		if len(ids) != 3 {
			t.Errorf("want all 3 entities (no gating), got %d: %v", len(ids), ids)
		}
	})

	t.Run("ListRelations yields every relation", func(t *testing.T) {
		var n int
		for _, err := range r.ListRelations(ctx, store.RelationQuery{}) {
			if err != nil {
				t.Fatalf("ListRelations: %v", err)
			}
			n++
		}
		if n != 1 {
			t.Errorf("want 1 relation, got %d", n)
		}
	})
}

// TestUnrestricted_ExposesOnlyTheReadSurface pins the deliberate
// narrowing. The wrapper must expose EXACTLY the three script-read methods
// — no writes, no admin surface — so that "ungated" cannot quietly come to
// mean "ungated and writable".
//
// The method set is asserted by name rather than via a `store.Store` type
// assertion: store.Store has ~10 methods, so an assertion against it only
// fires if someone re-embeds the entire store, and would stay silent while
// a single write method was bolted on. Enumerating is the stricter check —
// adding ANY method fails this test and forces a deliberate decision.
func TestUnrestricted_ExposesOnlyTheReadSurface(t *testing.T) {
	want := map[string]bool{
		"GetEntity": true, "ListEntities": true, "ListRelations": true,
	}

	typ := reflect.TypeOf(visibility.Unrestricted(seedStore(t)))
	got := make(map[string]bool, typ.NumMethod())
	for m := range typ.Methods() {
		got[m.Name] = true
	}

	for name := range got {
		if !want[name] {
			t.Errorf("UnrestrictedReader gained method %q — the ungated reader "+
				"must stay a three-method READ surface; a write or admin method "+
				"here turns every ungated wiring site into a wider capability", name)
		}
	}
	for name := range want {
		if !got[name] {
			t.Errorf("UnrestrictedReader is missing %q — it no longer satisfies "+
				"the Lua read surface structurally", name)
		}
	}

	// It must also not be usable anywhere a full store is expected.
	var r any = visibility.Unrestricted(seedStore(t))
	if _, ok := r.(store.Store); ok {
		t.Error("UnrestrictedReader satisfies store.Store — it has widened back " +
			"into a full store handle and no longer narrows the surface")
	}
}

// TestUnrestricted_NilStorePanics pins the wiring-mistake behavior.
//
// This MUST NOT go back to "return nil for a nil store". That looks safer
// and is the opposite: a nil *UnrestrictedReader assigned into the
// lua.EntityReader interface field yields a NON-nil interface (typed nil),
// so lua's `VisibleReader == nil` deny guard is skipped and the first read
// nil-derefs — a process-killing panic at request time instead of the
// clean "no reader is configured" error. Verified empirically, not
// reasoned about. Panicking at construction is the loud, early failure.
func TestUnrestricted_NilStorePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Unrestricted(nil) did not panic — a nil store must fail " +
				"loudly at wiring time; returning a typed nil silently bypasses " +
				"lua's deny guard and nil-derefs on first read")
		}
	}()
	_ = visibility.Unrestricted(nil)
}

// TestUnrestricted_SatisfiesReadSurfaceNonNil pins the interface-level
// property the panic protects: a successfully constructed reader is a
// NON-nil interface value, and a caller's `== nil` deny check therefore
// correctly reports "wired". Declared locally because internal/visibility
// must not import internal/lua (arch-lint); the method set is identical to
// lua.EntityReader, which is what structural satisfaction relies on.
func TestUnrestricted_SatisfiesReadSurfaceNonNil(t *testing.T) {
	type entityReader interface {
		GetEntity(ctx context.Context, id string) (*entity.Entity, error)
		ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
		ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
	}

	// Through reflect, so the compiler cannot fold the nil check away: a
	// direct `r == nil` on a concrete-typed assignment is statically false
	// and the linter (correctly) rejects it. What is being pinned is that
	// the interface holds a non-nil POINTER — the property that would break
	// if Unrestricted ever returned a typed nil again.
	var r entityReader = visibility.Unrestricted(seedStore(t))
	if reflect.ValueOf(r).IsNil() {
		t.Fatal("the interface holds a NIL pointer — a caller's `== nil` deny " +
			"check would report 'wired' and the first read would nil-deref")
	}
	if _, err := r.GetEntity(context.Background(), "TKT-1"); err != nil {
		t.Errorf("read through the structural interface failed: %v", err)
	}
}
