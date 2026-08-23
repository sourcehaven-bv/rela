package storetest

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunStateTests is the content-states conformance suite (TKT-DOFYR1):
// addressing by (id, pointer), the write-path row-family invariants,
// default-vs-AllStates query semantics, relation tail pointers in both
// match implementations, and the delete/rename cascades over a state
// family. These cases define the contract BEFORE the second backend
// implements — they are what keep storeutil.MatchRelation and pgstore's
// relationWhere from drifting.
func RunStateTests(t *testing.T, f Factory) {
	newState := func(t *testing.T, id, typ, ptr, title string) *entity.Entity {
		t.Helper()
		e := entity.New(id, typ)
		if ptr != "" {
			p, err := entity.ParsePointer(ptr)
			require.NoError(t, err)
			e.Pointer = p
		}
		e.SetString("title", title)
		return e
	}
	mustCreate := func(t *testing.T, s store.Store, e *entity.Entity) {
		t.Helper()
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
	ptr := func(t *testing.T, v string) entity.Pointer {
		t.Helper()
		p, err := entity.ParsePointer(v)
		require.NoError(t, err)
		return p
	}

	t.Run("AddressByPair", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-1", "page", "", "default face"))
		mustCreate(t, s, newState(t, "PAGE-1", "page", "draft", "draft face"))

		// Bare id = the default state.
		got, err := s.GetEntity(ctx(), "PAGE-1")
		require.NoError(t, err)
		assert.Equal(t, "default face", got.GetString("title"))
		assert.True(t, got.Pointer.IsDefault())

		// The pair addresses the state; the id stays bare on the result.
		draft, err := s.GetEntityState(ctx(), "PAGE-1", ptr(t, "draft"))
		require.NoError(t, err)
		assert.Equal(t, "PAGE-1", draft.ID, "the joined form must never leak into the id")
		assert.Equal(t, ptr(t, "draft"), draft.Pointer)
		assert.Equal(t, "draft face", draft.GetString("title"))

		// GetEntityState with the zero pointer ≡ GetEntity.
		def, err := s.GetEntityState(ctx(), "PAGE-1", "")
		require.NoError(t, err)
		assert.Equal(t, "default face", def.GetString("title"))

		// A missing state is ErrNotFound even though siblings exist.
		_, err = s.GetEntityState(ctx(), "PAGE-1", ptr(t, "published"))
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("PointerIsOpaqueToTheStore", func(t *testing.T) {
		// The store equality-matches pointer values, never inspects
		// them: an unusual-but-canonical coordinate round-trips
		// unchanged (the multi-axis guarantee, design doc §3.1).
		s := f(t)
		unusual := ptr(t, "x9-z2-a0")
		mustCreate(t, s, newState(t, "PAGE-2", "page", "", "default"))
		e := newState(t, "PAGE-2", "page", "", "odd face")
		e.Pointer = unusual
		mustCreate(t, s, e)

		got, err := s.GetEntityState(ctx(), "PAGE-2", unusual)
		require.NoError(t, err)
		assert.Equal(t, unusual, got.Pointer)
		assert.Equal(t, "odd face", got.GetString("title"))
	})

	t.Run("WriteInvariants", func(t *testing.T) {
		t.Run("HeadlessStateRejected", func(t *testing.T) {
			s := f(t)
			err := s.CreateEntity(ctx(), newState(t, "PAGE-3", "page", "draft", "no default"))
			require.Error(t, err, "a non-default state must not exist without the default row")
		})

		t.Run("TypeMismatchRejected", func(t *testing.T) {
			s := f(t)
			mustCreate(t, s, newState(t, "PAGE-4", "page", "", "default"))
			err := s.CreateEntity(ctx(), newState(t, "PAGE-4", "ticket", "draft", "wrong type"))
			require.Error(t, err, "states share their family's type")
		})

		t.Run("DuplicateStateConflicts", func(t *testing.T) {
			s := f(t)
			mustCreate(t, s, newState(t, "PAGE-5", "page", "", "default"))
			mustCreate(t, s, newState(t, "PAGE-5", "page", "draft", "draft"))
			err := s.CreateEntity(ctx(), newState(t, "PAGE-5", "page", "draft", "again"))
			assert.ErrorIs(t, err, store.ErrConflict)
		})

		t.Run("UpdateMissingStateNotFound", func(t *testing.T) {
			s := f(t)
			mustCreate(t, s, newState(t, "PAGE-6", "page", "", "default"))
			err := s.UpdateEntity(ctx(), newState(t, "PAGE-6", "page", "draft", "phantom"))
			assert.ErrorIs(t, err, store.ErrNotFound)
		})

		t.Run("UpdateStateCannotRetype", func(t *testing.T) {
			s := f(t)
			mustCreate(t, s, newState(t, "PAGE-7", "page", "", "default"))
			mustCreate(t, s, newState(t, "PAGE-7", "page", "draft", "draft"))
			err := s.UpdateEntity(ctx(), newState(t, "PAGE-7", "ticket", "draft", "retyped"))
			require.Error(t, err)
		})
	})

	t.Run("QueryScope", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-8", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-8", "page", "draft", "draft"))
		mustCreate(t, s, newState(t, "PAGE-9", "page", "", "other"))

		// The zero-value query is today's semantics: default states only.
		defaults := collectIter(t, s.ListEntities(ctx(), store.EntityQuery{Type: "page"}))
		require.Len(t, defaults, 2)
		for _, e := range defaults {
			assert.True(t, e.Pointer.IsDefault())
		}
		n, err := s.CountEntities(ctx(), store.EntityQuery{Type: "page"})
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		// AllStates is raw storage truth: every state row.
		all := collectIter(t, s.ListEntities(ctx(), store.EntityQuery{Type: "page", AllStates: true}))
		assert.Len(t, all, 3)
		n, err = s.CountEntities(ctx(), store.EntityQuery{Type: "page", AllStates: true})
		require.NoError(t, err)
		assert.Equal(t, 3, n)

		// IDs filters on the BARE id, so IDs+AllStates selects the family.
		family := collectIter(t, s.ListEntities(ctx(), store.EntityQuery{IDs: []string{"PAGE-8"}, AllStates: true}))
		assert.Len(t, family, 2)
	})

	t.Run("DefaultWorldAggregates", func(t *testing.T) {
		// PropertyValues and HighestID are default-world aggregates.
		// The count-ORDER assertion is the load-bearing part: with one
		// "open" default and two "closed" defaults, "closed" must sort
		// first — two states carrying "open" would flip that order if
		// states leaked into the counts.
		s := f(t)
		def := newState(t, "PAGE-20", "page", "", "default")
		def.SetString("status", "open")
		mustCreate(t, s, def)
		for _, p := range []string{"draft", "review"} {
			st := newState(t, "PAGE-20", "page", p, "state "+p)
			st.SetString("status", "open")
			mustCreate(t, s, st)
		}
		for _, id := range []string{"PAGE-21", "PAGE-22"} {
			e := newState(t, id, "page", "", "closed one")
			e.SetString("status", "closed")
			mustCreate(t, s, e)
		}

		vals, err := s.PropertyValues(ctx(), "status", 0)
		require.NoError(t, err)
		assert.Equal(t, []string{"closed", "open"}, vals,
			"states must not inflate suggestion counts (open would outrank closed)")

		high, err := s.HighestID(ctx(), "PAGE")
		require.NoError(t, err)
		assert.Equal(t, 22, high)
	})

	t.Run("RelationTails", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-10", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-10", "page", "draft", "draft"))
		mustCreate(t, s, newState(t, "SPEC-1", "page", "", "target"))

		// Same triple, two tails: two distinct relations.
		_, err := s.CreateRelation(ctx(), "PAGE-10", "references", "SPEC-1", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "PAGE-10", "references", "SPEC-1",
			&store.RelationData{FromPointer: ptr(t, "draft")})
		require.NoError(t, err)
		// The state-tailed duplicate conflicts like any relation.
		_, err = s.CreateRelation(ctx(), "PAGE-10", "references", "SPEC-1",
			&store.RelationData{FromPointer: ptr(t, "draft")})
		assert.ErrorIs(t, err, store.ErrConflict)

		// nil FromPointer = unfiltered (today's behavior): both edges.
		both := collectRelations(t, s, store.RelationQuery{From: "PAGE-10", Type: "references"})
		require.Len(t, both, 2)

		// A non-nil zero pointer matches the default tail only.
		var zero entity.Pointer
		defOnly := collectRelations(t, s, store.RelationQuery{
			From: "PAGE-10", Type: "references", FromPointer: &zero,
		})
		require.Len(t, defOnly, 1)
		assert.True(t, defOnly[0].FromPointer.IsDefault())

		// A pointer value matches that tail exactly — including through
		// the fs INDEXED query path (From+Type set), the silent-miss
		// class the relationMeta index fix guards.
		draft := ptr(t, "draft")
		draftOnly := collectRelations(t, s, store.RelationQuery{
			From: "PAGE-10", Type: "references", FromPointer: &draft,
		})
		require.Len(t, draftOnly, 1)
		assert.Equal(t, draft, draftOnly[0].FromPointer)

		// CountRelations agrees with ListRelations.
		n, err := s.CountRelations(ctx(), store.RelationQuery{From: "PAGE-10", Type: "references", FromPointer: &draft})
		require.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("HyphenBearingIDsWithPointerTails", func(t *testing.T) {
		// Base ids may contain single hyphens freely; what keeps the
		// relation key/filename parse unambiguous is that BOTH grammars
		// forbid "--" (ValidateID's no-consecutive-hyphens rule and the
		// pointer grammar's single-hyphen-runs rule). A hyphenated id
		// with a hyphenated pointer must round-trip through the FROM
		// slot serialization intact.
		s := f(t)
		mustCreate(t, s, newState(t, "MY-PAGE-1", "page", "", "default"))
		mustCreate(t, s, newState(t, "MY-PAGE-1", "page", "draft-nl", "state"))
		mustCreate(t, s, newState(t, "OTHER-DOC-2", "page", "", "target"))

		fp := ptr(t, "draft-nl")
		_, err := s.CreateRelation(ctx(), "MY-PAGE-1", "refers-to", "OTHER-DOC-2",
			&store.RelationData{FromPointer: fp})
		require.NoError(t, err)

		got := collectRelations(t, s, store.RelationQuery{From: "MY-PAGE-1", FromPointer: &fp})
		require.Len(t, got, 1)
		assert.Equal(t, "MY-PAGE-1", got[0].From)
		assert.Equal(t, fp, got[0].FromPointer)
		assert.Equal(t, "OTHER-DOC-2", got[0].To)
	})

	t.Run("DeleteCascadesTheFamily", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-11", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-11", "page", "draft", "draft"))
		mustCreate(t, s, newState(t, "SPEC-2", "page", "", "target"))
		_, err := s.CreateRelation(ctx(), "PAGE-11", "references", "SPEC-2", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "PAGE-11", "references", "SPEC-2",
			&store.RelationData{FromPointer: ptr(t, "draft")})
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "SPEC-2", "links", "PAGE-11", nil)
		require.NoError(t, err)

		res, err := s.DeleteEntity(ctx(), "PAGE-11", true)
		require.NoError(t, err)
		assert.Len(t, res.DeletedEntities, 2, "both states deleted")
		assert.Len(t, res.DeletedRelations, 3, "edges of every tail plus incoming")

		_, err = s.GetEntity(ctx(), "PAGE-11")
		assert.ErrorIs(t, err, store.ErrNotFound)
		_, err = s.GetEntityState(ctx(), "PAGE-11", ptr(t, "draft"))
		assert.ErrorIs(t, err, store.ErrNotFound)
		left := collectRelations(t, s, store.RelationQuery{EntityID: "PAGE-11"})
		assert.Empty(t, left)
	})

	// --- Per-face delete (TKT-C1XUA8) -----------------------------------
	//
	// The contract these pin is the OPPOSITE of DeleteCascadesTheFamily
	// above, and the pair is deliberate: DeleteEntity addresses the bare id
	// and sweeps the family; DeleteEntityState addresses one face and leaves
	// the family standing. A backend that implemented the second as the
	// first would pass neither.

	t.Run("DeleteStateLeavesSiblingFacesStanding", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-20", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-20", "page", "draft", "draft"))
		mustCreate(t, s, newState(t, "PAGE-20", "page", "published", "published"))

		res, err := s.DeleteEntityState(ctx(), "PAGE-20", ptr(t, "draft"))
		require.NoError(t, err)
		require.Len(t, res.DeletedEntities, 1, "exactly one face deleted")
		assert.Equal(t, ptr(t, "draft"), res.DeletedEntities[0].Pointer)

		_, err = s.GetEntityState(ctx(), "PAGE-20", ptr(t, "draft"))
		assert.ErrorIs(t, err, store.ErrNotFound, "the discarded face is gone")

		// The siblings must be untouched, contents included — a backend that
		// deleted the family would fail here rather than in a count.
		def, err := s.GetEntity(ctx(), "PAGE-20")
		require.NoError(t, err)
		assert.Equal(t, "default", def.GetString("title"))
		pub, err := s.GetEntityState(ctx(), "PAGE-20", ptr(t, "published"))
		require.NoError(t, err)
		assert.Equal(t, "published", pub.GetString("title"))
	})

	t.Run("DeleteStateTakesItsOwnEdgesButNotInboundOnes", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-21", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-21", "page", "draft", "draft"))
		mustCreate(t, s, newState(t, "SPEC-5", "page", "", "target"))
		mustCreate(t, s, newState(t, "OTHER-5", "page", "", "other"))

		draft := ptr(t, "draft")
		// One edge per tail on the SAME triple: these are two relations.
		_, err := s.CreateRelation(ctx(), "PAGE-21", "references", "SPEC-5", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "PAGE-21", "references", "SPEC-5",
			&store.RelationData{FromPointer: draft})
		require.NoError(t, err)
		// Inbound: belongs to the ENTITY, not to the draft face (§2.3).
		_, err = s.CreateRelation(ctx(), "OTHER-5", "links", "PAGE-21", nil)
		require.NoError(t, err)

		res, err := s.DeleteEntityState(ctx(), "PAGE-21", draft)
		require.NoError(t, err)
		require.Len(t, res.DeletedRelations, 1,
			"only the draft face's own outgoing edge goes with it")
		assert.Equal(t, draft, res.DeletedRelations[0].FromPointer)

		// The default face's edge on the same triple SURVIVES. This is the
		// case the tail-dropping bug got wrong: it deleted this one.
		remaining := collectRelations(t, s, store.RelationQuery{From: "PAGE-21"})
		require.Len(t, remaining, 1)
		assert.True(t, remaining[0].FromPointer.IsDefault(),
			"the default face's edge must survive its sibling's deletion")

		// Inbound edges survive: an entity elsewhere still points here.
		inbound := collectRelations(t, s, store.RelationQuery{To: "PAGE-21"})
		assert.Len(t, inbound, 1, "heads are entity-level, so inbound edges survive")
	})

	t.Run("DeleteStateRefusesTheDefaultFaceWhileSiblingsRemain", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-22", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-22", "page", "draft", "draft"))

		// A family with no default row has no defined meaning, and world
		// fallback resolves against it. Refusing is not a limitation to route
		// around — deleting it would leave the remaining faces unreachable by
		// every `otherwise: default` reader.
		_, err := s.DeleteEntityState(ctx(), "PAGE-22", "")
		require.Error(t, err)
		assert.ErrorIs(t, err, store.ErrInvalidQuery)

		_, err = s.GetEntity(ctx(), "PAGE-22")
		assert.NoError(t, err, "the refusal must not have deleted anything")
	})

	t.Run("DeleteStateOfTheLastFaceRemovesTheEntity", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-23", "page", "", "only"))

		// The default face is deletable when it is the ONLY face: no sibling
		// is orphaned, so the refusal above does not apply.
		res, err := s.DeleteEntityState(ctx(), "PAGE-23", "")
		require.NoError(t, err)
		assert.Len(t, res.DeletedEntities, 1)

		_, err = s.GetEntity(ctx(), "PAGE-23")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteStateEmitsOnlyItsOwnFaceEvent", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-28", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-28", "page", "draft", "draft"))

		events, cancel := s.Subscribe(16)
		defer cancel()
		_, err := s.DeleteEntityState(ctx(), "PAGE-28", ptr(t, "draft"))
		require.NoError(t, err)

		// EXACTLY ONE delete event, carrying the discarded face's pointer.
		// A backend that emitted the family's events here would be telling
		// every subscriber the whole entity is gone while the default face
		// still exists — silent, and invisible until someone read the entity
		// through a derived index.
		select {
		case ev := <-events:
			assert.Equal(t, store.EventEntityDeleted, ev.Op)
			assert.Equal(t, ptr(t, "draft"), ev.Pointer,
				"the event must name the face that was deleted")
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the face-delete event")
		}
		select {
		case ev := <-events:
			t.Errorf("a per-face delete must emit ONE event, got a second: %+v", ev)
		case <-time.After(100 * time.Millisecond):
			// Correct: nothing further.
		}
	})

	t.Run("DeleteStateNotFound", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-24", "page", "", "default"))

		_, err := s.DeleteEntityState(ctx(), "PAGE-24", ptr(t, "nosuchface"))
		assert.ErrorIs(t, err, store.ErrNotFound)
		_, err = s.DeleteEntityState(ctx(), "NO-SUCH-ENTITY", ptr(t, "draft"))
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	// --- Per-tail relation delete (TKT-C1XUA8) ---------------------------

	t.Run("DeleteRelationStateAddressesTheTailNotTheTriple", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-25", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-25", "page", "published", "published"))
		mustCreate(t, s, newState(t, "SPEC-6", "page", "", "target"))

		pub := ptr(t, "published")
		_, err := s.CreateRelation(ctx(), "PAGE-25", "references", "SPEC-6", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "PAGE-25", "references", "SPEC-6",
			&store.RelationData{FromPointer: pub})
		require.NoError(t, err)

		// Delete the PUBLISHED-tail edge. The regression this guards: every
		// backend's DeleteRelation is default-tail-only, so a caller dropping
		// the tail deleted the DEFAULT edge and reported success while the
		// published edge survived — the wrong edge, silently.
		require.NoError(t, s.DeleteRelationState(ctx(), "PAGE-25", pub, "references", "SPEC-6"))

		remaining := collectRelations(t, s, store.RelationQuery{From: "PAGE-25"})
		require.Len(t, remaining, 1)
		assert.True(t, remaining[0].FromPointer.IsDefault(),
			"the default-tail edge must survive deletion of its published-tail sibling")
	})

	t.Run("DeleteRelationStateZeroPointerMatchesDeleteRelation", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-26", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-26", "page", "draft", "draft"))
		mustCreate(t, s, newState(t, "SPEC-7", "page", "", "target"))

		_, err := s.CreateRelation(ctx(), "PAGE-26", "references", "SPEC-7", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "PAGE-26", "references", "SPEC-7",
			&store.RelationData{FromPointer: ptr(t, "draft")})
		require.NoError(t, err)

		// The zero pointer is the general form's default-tail address, so it
		// must behave exactly as DeleteRelation does — same edge, same result.
		require.NoError(t, s.DeleteRelationState(ctx(), "PAGE-26", "", "references", "SPEC-7"))

		remaining := collectRelations(t, s, store.RelationQuery{From: "PAGE-26"})
		require.Len(t, remaining, 1)
		assert.Equal(t, ptr(t, "draft"), remaining[0].FromPointer)
	})

	t.Run("DeleteRelationStateNotFound", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-27", "page", "", "default"))
		mustCreate(t, s, newState(t, "SPEC-8", "page", "", "target"))
		_, err := s.CreateRelation(ctx(), "PAGE-27", "references", "SPEC-8", nil)
		require.NoError(t, err)

		// A tail with no edge is absent, NOT "close enough" to the default
		// edge that does exist.
		err = s.DeleteRelationState(ctx(), "PAGE-27", ptr(t, "draft"), "references", "SPEC-8")
		assert.ErrorIs(t, err, store.ErrNotFound)
		assert.Len(t, collectRelations(t, s, store.RelationQuery{From: "PAGE-27"}), 1,
			"the miss must not have deleted the default-tail edge")
	})

	t.Run("RenameCascadesTheFamily", func(t *testing.T) {
		s := f(t)
		mustCreate(t, s, newState(t, "PAGE-12", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-12", "page", "draft", "draft"))
		mustCreate(t, s, newState(t, "SPEC-3", "page", "", "target"))
		_, err := s.CreateRelation(ctx(), "PAGE-12", "references", "SPEC-3",
			&store.RelationData{FromPointer: ptr(t, "draft")})
		require.NoError(t, err)

		res, err := s.RenameEntity(ctx(), "PAGE-12", "PAGE-99")
		require.NoError(t, err)
		assert.Equal(t, 1, res.RelationsUpdated)

		// Every state re-keyed onto the new id.
		def, err := s.GetEntity(ctx(), "PAGE-99")
		require.NoError(t, err)
		assert.Equal(t, "default", def.GetString("title"))
		draft, err := s.GetEntityState(ctx(), "PAGE-99", ptr(t, "draft"))
		require.NoError(t, err)
		assert.Equal(t, "draft", draft.GetString("title"))
		_, err = s.GetEntityState(ctx(), "PAGE-12", ptr(t, "draft"))
		assert.ErrorIs(t, err, store.ErrNotFound)

		// The tail pointer rode along.
		moved := collectRelations(t, s, store.RelationQuery{From: "PAGE-99", Type: "references"})
		require.Len(t, moved, 1)
		assert.Equal(t, ptr(t, "draft"), moved[0].FromPointer)

		// Any state of an existing entity blocks the rename target.
		mustCreate(t, s, newState(t, "PAGE-13", "page", "", "blocker"))
		_, err = s.RenameEntity(ctx(), "PAGE-99", "PAGE-13")
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	t.Run("EventsFirePerState", func(t *testing.T) {
		s := f(t)
		events, cancel := s.Subscribe(16)
		defer cancel()

		mustCreate(t, s, newState(t, "PAGE-14", "page", "", "default"))
		mustCreate(t, s, newState(t, "PAGE-14", "page", "draft", "draft"))
		_, err := s.DeleteEntity(ctx(), "PAGE-14", true)
		require.NoError(t, err)

		// Bounded blocking receive: backends with async delivery (the pg
		// change-feed bridge) may not have all four buffered yet.
		var pointers []string
		timeout := time.After(5 * time.Second)
		for range 4 {
			select {
			case ev := <-events:
				pointers = append(pointers, string(ev.Pointer))
			case <-timeout:
				t.Fatalf("timed out waiting for events; got %v", pointers)
			}
		}
		// Two creates + two deletes, each carrying its state's pointer.
		sort.Strings(pointers)
		assert.Equal(t, []string{"", "", "draft", "draft"}, pointers)
	})
}

// collectRelations drains ListRelations for q.
func collectRelations(t *testing.T, s store.Store, q store.RelationQuery) []*entity.Relation {
	t.Helper()
	var out []*entity.Relation
	for r, err := range s.ListRelations(ctx(), q) {
		require.NoError(t, err)
		out = append(out, r)
	}
	return out
}
