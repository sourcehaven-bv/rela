package storetest

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// RunInstantTests is the conformance suite for point-in-time reads
// ([store.BindReadInstant], TKT-6KLUT3U). It runs for a backend that declares
// Capabilities.ReadInstant, which needs Versioning and SweepNow too.
//
// The suite drives captures the way production does: the sweep for creates
// and updates, the synchronous writer for renames and deletes. Each case
// takes its instant from a row the store wrote, so database and test clocks
// never have to agree.
func RunInstantTests(t *testing.T, f Factory, sweepNow func(t *testing.T, s store.Store)) {
	t.Run("EntityCreatedAfterIsAbsent", func(t *testing.T) {
		s := f(t)
		at := createAndMark(t, s, "FEAT-1", "", "One")
		createEntity(t, s, "FEAT-2", "", "Two")
		sweepNow(t, s)

		require.Equal(t, []string{"FEAT-1"}, idsAt(t, s, at))
		_, err := s.GetEntity(atCtx(t, s, at), entity.Ref{ID: "FEAT-2"})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("EntityUpdatedAfterReadsOldContent", func(t *testing.T) {
		s := f(t)
		at := createAndMark(t, s, "FEAT-1", "", "Old")
		sweepNow(t, s)
		waitPast(at)
		updateTitle(t, s, "FEAT-1", "New")

		require.Equal(t, "Old", titleAt(t, s, at),
			"before the sweep captured the update")
		sweepNow(t, s)
		require.Equal(t, "Old", titleAt(t, s, at),
			"after the sweep captured the update")
		require.Equal(t, "New", titleAt(t, s, time.Now().Add(time.Hour)))
	})

	t.Run("CaptureDelayDoesNotMoveContent", func(t *testing.T) {
		// The update is swept long after it was written. The state just
		// after the write must already show it.
		s := f(t)
		createAndMark(t, s, "FEAT-1", "", "Old")
		sweepNow(t, s)
		waitPast(time.Now())
		updateTitle(t, s, "FEAT-1", "New")
		after := markOf(t, s, "FEAT-1", "")
		waitPast(after)
		sweepNow(t, s)
		updateTitle(t, s, "FEAT-1", "Newer")

		require.Equal(t, "New", titleAt(t, s, after))
	})

	t.Run("EntityDeletedAfterIsPresent", func(t *testing.T) {
		s := f(t)
		at := createAndMark(t, s, "FEAT-1", "", "Doomed")
		sweepNow(t, s)
		waitPast(at)
		deleteWithVersion(t, s, "FEAT-1")
		gone := time.Now()
		waitPast(gone)

		require.Equal(t, "Doomed", titleAt(t, s, at))
		require.Empty(t, idsAt(t, s, time.Now()), "absent once the delete is in the past")
	})

	t.Run("EntityRenamedAfterKeepsOldID", func(t *testing.T) {
		s := f(t)
		at := createAndMark(t, s, "FEAT-1", "", "Renamed")
		sweepNow(t, s)
		waitPast(at)
		_, err := s.RenameFamily(ctx(), "FEAT-1", "FEAT-9")
		require.NoError(t, err)
		writeVersion(t, versionsOf(t, s), store.VersionInput{
			EntityID: "FEAT-9", Op: store.VersionOpRename, PrevID: "FEAT-1",
			Type: "feature", Properties: map[string]any{"title": "Renamed"},
		})
		waitPast(time.Now())

		require.Equal(t, []string{"FEAT-1"}, idsAt(t, s, at))
		require.Equal(t, []string{"FEAT-9"}, idsAt(t, s, time.Now()))
	})

	t.Run("FacePublishedAfterIsAbsent", func(t *testing.T) {
		s := f(t)
		at := createAndMark(t, s, "DOC-1", "draft", "Draft")
		createEntity(t, s, "DOC-1", "published", "Published")
		sweepNow(t, s)

		var faces []string
		for e, err := range s.ListEntities(atCtx(t, s, at), store.EntityQuery{Faces: store.AllFaces()}) {
			require.NoError(t, err)
			faces = append(faces, string(e.Face))
		}
		require.Equal(t, []string{"draft"}, faces)
		_, err := s.GetEntity(atCtx(t, s, at), entity.Ref{ID: "DOC-1", Face: "published"})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("HeadersGraphQueryAndCountsReadTheInstant", func(t *testing.T) {
		s := f(t)
		createEntity(t, s, "FEAT-1", "", "One")
		at := createAndMark(t, s, "FEAT-2", "", "Two")
		sweepNow(t, s)
		waitPast(at)
		updateTitle(t, s, "FEAT-1", "Changed")
		createEntity(t, s, "FEAT-3", "", "Three")
		sweepNow(t, s)

		c := atCtx(t, s, at)
		q := store.GraphQuery{
			EntityType: "feature",
			Props:      []store.PropPredicate{{Property: "title", Op: store.PropEqual, Value: "One", Scalar: true}},
			Faces:      store.InWorld(store.TrivialScope()),
		}
		var got []string
		for h, err := range store.GraphQueryHeaders(c, s, q) {
			require.NoError(t, err)
			got = append(got, h.ID)
		}
		require.Equal(t, []string{"FEAT-1"}, got, "the query matches the title the row had at the instant")

		n, err := store.CountMatched(c, s, store.GraphQuery{EntityType: "feature", Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		require.Equal(t, 2, n)

		n, err = s.CountEntities(c, store.EntityQuery{Type: "feature", Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		require.Equal(t, 2, n)

		var headers []string
		for h, err := range store.ListEntityHeaders(c, s, store.EntityQuery{Type: "feature", Faces: store.InWorld(store.TrivialScope())}) {
			require.NoError(t, err)
			title, _ := h.Properties["title"].(string)
			headers = append(headers, h.ID+"="+title)
		}
		slices.Sort(headers)
		require.Equal(t, []string{"FEAT-1=One", "FEAT-2=Two"}, headers)
	})

	t.Run("Relations", func(t *testing.T) {
		s := f(t)
		createEntity(t, s, "FEAT-1", "", "One")
		createEntity(t, s, "FEAT-2", "", "Two")
		createEntity(t, s, "FEAT-3", "", "Three")
		kept := entity.RelationKey{From: "FEAT-1", Type: "depends-on", To: "FEAT-2"}
		_, err := s.CreateRelation(ctx(), kept, &store.RelationData{Content: "old"})
		require.NoError(t, err)
		doomed := entity.RelationKey{From: "FEAT-2", Type: "depends-on", To: "FEAT-3"}
		_, err = s.CreateRelation(ctx(), doomed, nil)
		require.NoError(t, err)
		sweepNow(t, s)
		at := relationMark(t, s, doomed)
		waitPast(at)

		_, err = s.UpdateRelation(ctx(), kept, store.RelationData{Content: "new"})
		require.NoError(t, err)
		require.NoError(t, versionsOf(t, s).WriteRelationVersion(ctx(), store.RelationVersionInput{
			Key: doomed, Op: store.VersionOpDelete, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
		}))
		require.NoError(t, s.DeleteRelation(ctx(), doomed))
		late := entity.RelationKey{From: "FEAT-3", Type: "depends-on", To: "FEAT-1"}
		_, err = s.CreateRelation(ctx(), late, nil)
		require.NoError(t, err)
		sweepNow(t, s)

		c := atCtx(t, s, at)
		var keys []string
		for r, err := range s.ListRelations(c, store.RelationQuery{}) {
			require.NoError(t, err)
			keys = append(keys, r.From+">"+r.To)
		}
		slices.Sort(keys)
		require.Equal(t, []string{"FEAT-1>FEAT-2", "FEAT-2>FEAT-3"}, keys)

		r, err := s.GetRelation(c, kept)
		require.NoError(t, err)
		require.Equal(t, "old", r.Content)
		_, err = s.GetRelation(c, late)
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("ACLGateReadsTodaysEdges", func(t *testing.T) {
		// DEC-TDEF3GD: the past is read with today's ACL. A gate predicate
		// (HasInbound / HasOutbound, an Any branch) reads the live edges, so
		// a role edge removed since T no longer grants a read at T, and one
		// added since does. A Related traversal is data and reads T.
		s := f(t)
		createEntity(t, s, "USR-1", "", "Principal")
		createEntity(t, s, "FEAT-1", "", "Lost")
		createEntity(t, s, "FEAT-2", "", "Gained")
		lost := entity.RelationKey{From: "USR-1", Type: "member-of", To: "FEAT-1"}
		_, err := s.CreateRelation(ctx(), lost, nil)
		require.NoError(t, err)
		sweepNow(t, s)
		at := relationMark(t, s, lost)
		waitPast(at)

		require.NoError(t, versionsOf(t, s).WriteRelationVersion(ctx(), store.RelationVersionInput{
			Key: lost, Op: store.VersionOpDelete, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
		}))
		require.NoError(t, s.DeleteRelation(ctx(), lost))
		_, err = s.CreateRelation(ctx(), entity.RelationKey{From: "USR-1", Type: "member-of", To: "FEAT-2"}, nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), entity.RelationKey{From: "FEAT-2", Type: "member-of", To: "USR-1"}, nil)
		require.NoError(t, err)
		sweepNow(t, s)

		c := atCtx(t, s, at)
		member := store.RelationPredicate{Endpoints: []string{"USR-1"}, OfTypes: []string{"member-of"}}
		world := store.InWorld(store.TrivialScope())
		headers := func(q store.GraphQuery) []string {
			t.Helper()
			var ids []string
			for h, err := range store.GraphQueryHeaders(c, s, q) {
				require.NoError(t, err)
				ids = append(ids, h.ID)
			}
			slices.Sort(ids)
			return ids
		}

		inbound := &member
		require.Equal(t, []string{"FEAT-2"},
			headers(store.GraphQuery{EntityType: "feature", HasInbound: inbound, Faces: world}),
			"HasInbound reads today's edges")
		require.Equal(t, []string{"FEAT-2"},
			headers(store.GraphQuery{EntityType: "feature", Faces: world,
				HasOutbound: &store.RelationPredicate{Endpoints: []string{"USR-1"}, OfTypes: []string{"member-of"}}}),
			"HasOutbound reads today's edges")
		require.Equal(t, []string{"FEAT-2"},
			headers(store.GraphQuery{EntityType: "feature", Faces: world,
				Any: []store.GraphBranch{{HasInbound: inbound}}}),
			"an Any branch reads today's edges")
		for _, q := range []store.GraphQuery{
			{EntityType: "feature", HasInbound: inbound, Faces: world},
			{EntityType: "feature", Faces: world, Any: []store.GraphBranch{{HasInbound: inbound}}},
		} {
			var naive []string
			for e, err := range graphquerynaive.Run(c, s, q) {
				require.NoError(t, err)
				naive = append(naive, e.ID)
			}
			require.Equal(t, []string{"FEAT-2"}, naive, "graphquerynaive's gate reads today's edges too")
		}
		faces, err := s.MatchingFaces(c, store.GraphQuery{EntityType: "feature", HasInbound: inbound, Faces: world},
			[]string{"FEAT-1", "FEAT-2"})
		require.NoError(t, err)
		require.Equal(t, map[string][]entity.Face{"FEAT-2": {""}}, faces)

		require.Equal(t, []string{"FEAT-1"},
			headers(store.GraphQuery{EntityType: "feature", Faces: world,
				Related: []store.DirectedRelation{{Incoming: true, Pred: member}}}),
			"a Related traversal reads the edges at the instant")
	})

	t.Run("ACLGateJudgesTheRowToday", func(t *testing.T) {
		// The gate reads no value of the candidate row, only today's edges
		// and endpoint rows by its id, so a row that still exists passes
		// or fails as it does today (DEC-TDEF3GD). An entity deleted since
		// has no edges today: it is present at the instant but no role
		// relation reaches it, so only an ungated read shows it.
		s := f(t)
		createEntity(t, s, "USR-1", "", "Principal")
		createEntity(t, s, "FEAT-1", "", "Deleted since")
		createEntity(t, s, "FEAT-2", "", "Kept")
		for _, to := range []string{"FEAT-1", "FEAT-2"} {
			_, err := s.CreateRelation(ctx(), entity.RelationKey{From: "USR-1", Type: "member-of", To: to}, nil)
			require.NoError(t, err)
		}
		sweepNow(t, s)
		at := time.Now()
		waitPast(at)
		deleteWithVersion(t, s, "FEAT-1")
		waitPast(time.Now())

		c := atCtx(t, s, at)
		world := store.InWorld(store.TrivialScope())
		member := &store.RelationPredicate{Endpoints: []string{"USR-1"}, OfTypes: []string{"member-of"}}
		ids := func(q store.GraphQuery) []string {
			t.Helper()
			var out []string
			for h, err := range store.GraphQueryHeaders(c, s, q) {
				require.NoError(t, err)
				out = append(out, h.ID)
			}
			slices.Sort(out)
			return out
		}
		require.Equal(t, []string{"FEAT-1", "FEAT-2", "USR-1"},
			ids(store.GraphQuery{EntityType: "feature", Faces: world}), "all three existed at the instant")
		require.Equal(t, []string{"FEAT-2"},
			ids(store.GraphQuery{EntityType: "feature", HasInbound: member, Faces: world}),
			"the deleted entity has no edge today, so the gate hides it")
		require.Equal(t, []string{"FEAT-1", "FEAT-2"},
			ids(store.GraphQuery{EntityType: "feature", Faces: world,
				Related: []store.DirectedRelation{{Incoming: true, Pred: *member}}}),
			"a Related traversal still sees the edge as it stood")
	})

	t.Run("GateFoldedIntoATraversalReadsToday", func(t *testing.T) {
		// A related(...) traversal is data and reads the instant; the read
		// gate folded onto its endpoint (RelationPredicate.Gate) reads
		// today's edges (DEC-TDEF3GD). FEAT-2 reaches FEAT-1, whose gate
		// edge was removed since; FEAT-3 reaches FEAT-4, whose gate edge
		// stands; FEAT-5's hop to FEAT-4 was added since. Both the store's
		// own query and graphquerynaive (sqlite's fallback) agree.
		s := f(t)
		for _, id := range []string{"USR-1", "FEAT-1", "FEAT-2", "FEAT-3", "FEAT-4", "FEAT-5"} {
			createEntity(t, s, id, "", id)
		}
		lost := entity.RelationKey{From: "USR-1", Type: "member-of", To: "FEAT-1"}
		for _, k := range []entity.RelationKey{
			lost,
			{From: "USR-1", Type: "member-of", To: "FEAT-4"},
			{From: "FEAT-2", Type: "depends-on", To: "FEAT-1"},
			{From: "FEAT-3", Type: "depends-on", To: "FEAT-4"},
		} {
			_, err := s.CreateRelation(ctx(), k, nil)
			require.NoError(t, err)
		}
		sweepNow(t, s)
		at := relationMark(t, s, entity.RelationKey{From: "FEAT-3", Type: "depends-on", To: "FEAT-4"})
		waitPast(at)
		require.NoError(t, versionsOf(t, s).WriteRelationVersion(ctx(), store.RelationVersionInput{
			Key: lost, Op: store.VersionOpDelete, SchemaHash: "schema-1", Projection: []byte(`{"v":1}`),
		}))
		require.NoError(t, s.DeleteRelation(ctx(), lost))
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: "FEAT-5", Type: "depends-on", To: "FEAT-4"}, nil)
		require.NoError(t, err)
		sweepNow(t, s)

		c := atCtx(t, s, at)
		traversal := func(gate bool) store.GraphQuery {
			return store.GraphQuery{EntityType: "feature", Faces: store.AllFaces(),
				Related: []store.DirectedRelation{{Pred: store.RelationPredicate{
					OfTypes: []string{"depends-on"},
					EndpointMatch: &store.EndpointPredicate{EntityType: "feature",
						HasInbound: &store.RelationPredicate{
							Endpoints: []string{"USR-1"}, OfTypes: []string{"member-of"}, Gate: gate}},
				}}}}
		}
		both := func(q store.GraphQuery) (stored, naive []string) {
			t.Helper()
			for h, err := range store.GraphQueryHeaders(c, s, q) {
				require.NoError(t, err)
				stored = append(stored, h.ID)
			}
			for e, err := range graphquerynaive.Run(c, s, q) {
				require.NoError(t, err)
				naive = append(naive, e.ID)
			}
			slices.Sort(stored)
			slices.Sort(naive)
			return stored, naive
		}
		stored, naive := both(traversal(true))
		require.Equal(t, []string{"FEAT-3"}, stored, "the gate reads today; the hop reads the instant")
		require.Equal(t, stored, naive, "graphquerynaive agrees")
		stored, naive = both(traversal(false))
		require.Equal(t, []string{"FEAT-2", "FEAT-3"}, stored, "an unmarked endpoint predicate reads the instant")
		require.Equal(t, stored, naive, "graphquerynaive agrees")
	})

	t.Run("SearchFindsTheTextAtTheInstant", func(t *testing.T) {
		s := f(t)
		vs, ok := s.(search.VisibleSearcher)
		if !ok {
			// sqlitestore's search reads the FTS index, which has no history;
			// versioning.md lists the search backend as a read off the seam.
			t.Skip("the store does not search through its own read seam")
		}
		at := createAndMark(t, s, "FEAT-1", "", "Aardvark")
		createEntity(t, s, "FEAT-2", "", "Aardwolf")
		sweepNow(t, s)
		waitPast(markOf(t, s, "FEAT-2", ""))
		updateTitle(t, s, "FEAT-1", "Zebra")
		sweepNow(t, s)

		c := atCtx(t, s, at)
		scope := map[string]search.TypeScope{"feature": {AllowAll: true}}
		find := func(text string, limit int) []string {
			t.Helper()
			var ids []string
			for h, err := range vs.SearchVisible(c, search.Query{Text: text, Limit: limit, World: store.TrivialScope()}, scope) {
				require.NoError(t, err)
				ids = append(ids, h.ID)
			}
			return ids
		}
		require.Equal(t, []string{"FEAT-1"}, find("aardvark", 0), "found by its title at the instant")
		require.Empty(t, find("zebra", 0), "not found by the title it got later")
		require.Equal(t, []string{"FEAT-1"}, find("aard", 1), "a ranked, limited search pages over the snapshot")
	})

	t.Run("HeaderPageIsScaleFree", func(t *testing.T) {
		// The snapshot is part of the statement, so a page read at an
		// instant costs one call and a page of rows at any store size.
		storeSize := func(n int) (Cost, string) {
			s := f(t)
			for i := range n {
				createEntity(t, s, fmt.Sprintf("FEAT-%04d", i), "", "Padding")
			}
			sweepNow(t, s)
			at := markOf(t, s, fmt.Sprintf("FEAT-%04d", n-1), "")
			waitPast(at)
			updateTitle(t, s, "FEAT-0000", "Changed")
			sweepNow(t, s)

			cs := NewCounting(s)
			page, err := store.ListEntityHeadersPage(atCtx(t, cs, at), cs,
				store.EntityQuery{Type: "feature", Faces: store.InWorld(store.TrivialScope()), Limit: 10})
			require.NoError(t, err)
			require.Len(t, page.Items, 10)
			require.Equal(t, "Padding", page.Items[0].Properties["title"])
			return cs.Cost(), cs.String()
		}
		AssertScaleFree(t, storeSize)
	})

	t.Run("WritesAreRefused", func(t *testing.T) {
		s := f(t)
		at := createAndMark(t, s, "FEAT-1", "", "One")
		c := atCtx(t, s, at)

		require.ErrorIs(t, s.CreateEntity(c, entity.New("FEAT-2", "feature")), store.ErrReadOnlyInstant)
		e := entity.New("FEAT-1", "feature")
		e.SetString("title", "Changed")
		require.ErrorIs(t, s.UpdateEntity(c, e), store.ErrReadOnlyInstant)
		_, err := s.DeleteFamily(c, "FEAT-1", true)
		require.ErrorIs(t, err, store.ErrReadOnlyInstant)
		_, err = s.CreateRelation(c, entity.RelationKey{From: "FEAT-1", Type: "x", To: "FEAT-1"}, nil)
		require.ErrorIs(t, err, store.ErrReadOnlyInstant)
		require.ErrorIs(t, s.Tx(c, func(store.Store) error { return nil }), store.ErrReadOnlyInstant)

		require.Equal(t, "One", titleAt(t, s, time.Now().Add(time.Hour)), "nothing was written")
	})

	t.Run("ZeroInstantIsRejected", func(t *testing.T) {
		_, err := store.BindReadInstant(ctx(), f(t), time.Time{})
		require.ErrorIs(t, err, store.ErrInvalidQuery)
	})
}

// RunInstantUnsupportedTests pins that a backend without version history
// refuses a read instant rather than answering with live data.
func RunInstantUnsupportedTests(t *testing.T, f Factory) {
	t.Run("BindIsRefused", func(t *testing.T) {
		_, err := store.BindReadInstant(ctx(), f(t), time.Now())
		require.ErrorIs(t, err, store.ErrInstantUnsupported)
	})
}

// atCtx binds a read instant for s.
func atCtx(t *testing.T, s store.Store, at time.Time) context.Context {
	t.Helper()
	c, err := store.BindReadInstant(ctx(), s, at)
	require.NoError(t, err)
	return c
}

func createEntity(t *testing.T, s store.Store, id string, face entity.Face, title string) {
	t.Helper()
	e := entity.New(id, "feature")
	e.Face = face
	e.SetString("title", title)
	require.NoError(t, s.CreateEntity(ctx(), e))
}

// createAndMark creates an entity and returns the instant it was written,
// as the store recorded it, after waiting until that instant has passed.
func createAndMark(t *testing.T, s store.Store, id string, face entity.Face, title string) time.Time {
	t.Helper()
	createEntity(t, s, id, face, title)
	at := markOf(t, s, id, face)
	waitPast(at)
	return at
}

// markOf is the stored write time of an entity face.
func markOf(t *testing.T, s store.Store, id string, face entity.Face) time.Time {
	t.Helper()
	e, err := s.GetEntity(ctx(), entity.Ref{ID: id, Face: face})
	require.NoError(t, err)
	require.False(t, e.UpdatedAt.IsZero(), "the backend must report when it wrote the row")
	return e.UpdatedAt
}

func relationMark(t *testing.T, s store.Store, k entity.RelationKey) time.Time {
	t.Helper()
	r, err := s.GetRelation(ctx(), k)
	require.NoError(t, err)
	require.False(t, r.UpdatedAt.IsZero())
	return r.UpdatedAt
}

// waitPast sleeps until the wall clock is safely after at, so the next write
// is stamped later than at.
func waitPast(at time.Time) {
	if d := time.Until(at.Add(5 * time.Millisecond)); d > 0 {
		time.Sleep(d)
	}
	time.Sleep(5 * time.Millisecond)
}

func updateTitle(t *testing.T, s store.Store, id, title string) {
	t.Helper()
	e, err := s.GetEntity(ctx(), entity.Ref{ID: id})
	require.NoError(t, err)
	e.SetString("title", title)
	require.NoError(t, s.UpdateEntity(ctx(), e))
}

func titleAt(t *testing.T, s store.Store, at time.Time) string {
	t.Helper()
	const id = "FEAT-1"
	e, err := s.GetEntity(atCtx(t, s, at), entity.Ref{ID: id})
	require.NoError(t, err)
	return e.GetString("title")
}

func idsAt(t *testing.T, s store.Store, at time.Time) []string {
	t.Helper()
	var ids []string
	for e, err := range s.ListEntities(atCtx(t, s, at), store.EntityQuery{Faces: store.InWorld(store.TrivialScope())}) {
		require.NoError(t, err)
		ids = append(ids, e.ID)
	}
	slices.Sort(ids)
	return ids
}
