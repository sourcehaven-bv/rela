package storetest

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// softDeleterOf returns the store's soft-delete capability or fails the test:
// a backend that declares Capabilities.SoftDelete must have it.
func softDeleterOf(t *testing.T, s store.Store) store.SoftDeleter {
	t.Helper()
	p, ok := s.(store.SoftDeleteProvider)
	require.True(t, ok, "store declared Capabilities.SoftDelete but does not implement store.SoftDeleteProvider")
	return p.SoftDelete()
}

// seedSoftDelete builds the fixture every soft-delete case starts from.
// FEAT-009 is the entity that gets marked: two faces, an outgoing and an
// incoming relation, and (when supported) an attachment. The highest FEAT
// number is deliberately the marked one, so HighestID shows it is still
// counted.
func seedSoftDelete(t *testing.T, s store.Store, attachments bool) {
	t.Helper()
	mk := func(id, typ, title string, face entity.Face) {
		e := entity.New(id, typ)
		e.Face = face
		e.SetString("title", title)
		e.SetString("status", "open")
		e.Content = "body of " + title
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
	mk("FEAT-009", "feature", "Marked", "")
	mk("FEAT-009", "feature", "Marked draft", "draft")
	mk("FEAT-002", "feature", "Neighbor", "")
	mk("REQ-001", "requirement", "Target", "")
	for _, r := range [][3]string{
		{"FEAT-009", "implements", "REQ-001"},
		{"FEAT-002", "depends-on", "FEAT-009"},
		{"FEAT-002", "implements", "REQ-001"},
	} {
		_, err := s.CreateRelation(ctx(), entity.RelationKey{From: r[0], Type: r[1], To: r[2]}, &store.RelationData{Content: "edge"})
		require.NoError(t, err)
	}
	if attachments {
		require.NoError(t, s.AttachFamilyFile(ctx(), "FEAT-009", "files", "a.txt", bytes.NewReader([]byte("hello"))))
	}
}

func entityIDs(t *testing.T, it func(func(*entity.Entity, error) bool)) []string {
	t.Helper()
	var ids []string
	for e, err := range it {
		require.NoError(t, err)
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	return ids
}

func relationKeys(t *testing.T, it func(func(*entity.Relation, error) bool)) []string {
	t.Helper()
	var keys []string
	for r, err := range it {
		require.NoError(t, err)
		keys = append(keys, r.From+"--"+r.Type+"--"+r.To)
	}
	sort.Strings(keys)
	return keys
}

// assertMarkedHidden runs every read method the store offers and checks that
// FEAT-009 and its relations are absent from all of them.
func assertMarkedHidden(t *testing.T, s store.Store, attachments bool) {
	t.Helper()
	c := ctx()

	_, err := s.GetEntity(c, entity.Ref{ID: "FEAT-009"})
	assert.ErrorIs(t, err, store.ErrNotFound, "GetEntity")
	_, err = s.GetEntity(c, entity.Ref{ID: "FEAT-009", Face: "draft"})
	assert.ErrorIs(t, err, store.ErrNotFound, "GetEntityState")

	live := []string{"FEAT-002", "REQ-001"}
	assert.Equal(t, live, entityIDs(t, s.ListEntities(c, store.EntityQuery{Faces: store.InWorld(store.TrivialScope())})), "ListEntities")
	assert.Equal(t, live, entityIDs(t, s.ListEntities(c, store.EntityQuery{Faces: store.AllFaces()})),
		"ListEntities AllStates")
	assert.Empty(t, entityIDs(t, s.ListEntities(c, store.EntityQuery{IDs: []string{"FEAT-009"}, Faces: store.AllFaces()})),
		"ListEntities by id")

	page, err := s.ListEntitiesPage(c, store.EntityQuery{Limit: 10, Faces: store.InWorld(store.TrivialScope())})
	require.NoError(t, err)
	assert.Len(t, page.Items, 2, "ListEntitiesPage")

	n, err := s.CountEntities(c, store.EntityQuery{Faces: store.InWorld(store.TrivialScope())})
	require.NoError(t, err)
	assert.Equal(t, 2, n, "CountEntities")
	n, err = s.CountEntities(c, store.EntityQuery{Type: "feature", Faces: store.AllFaces()})
	require.NoError(t, err)
	assert.Equal(t, 1, n, "CountEntities feature AllStates")

	var headerIDs []string
	for h, herr := range store.ListEntityHeaders(c, s, store.EntityQuery{Faces: store.AllFaces()}) {
		require.NoError(t, herr)
		headerIDs = append(headerIDs, h.ID)
	}
	sort.Strings(headerIDs)
	assert.Equal(t, live, headerIDs, "ListEntityHeaders")

	high, err := s.HighestID(c, "FEAT")
	require.NoError(t, err)
	assert.Equal(t, 9, high, "HighestID must still count a marked id, or the next id would reuse it")

	gq := store.GraphQuery{EntityType: "feature", Faces: store.InWorld(store.TrivialScope())}
	assert.Equal(t, []string{"FEAT-002"}, entityIDs(t, s.GraphQuery(c, gq)), "GraphQuery")
	matched, total, err := s.GraphCount(c, gq)
	require.NoError(t, err)
	assert.Equal(t, 1, matched, "GraphCount matched")
	assert.Equal(t, 1, total, "GraphCount total")
	cm, err := store.CountMatched(c, s, gq)
	require.NoError(t, err)
	assert.Equal(t, 1, cm, "CountMatched")
	if hq, ok := s.(store.GraphHeaderQueryer); ok {
		var ids []string
		for h, herr := range hq.GraphQueryHeaders(c, gq) {
			require.NoError(t, herr)
			ids = append(ids, h.ID)
		}
		assert.Equal(t, []string{"FEAT-002"}, ids, "GraphQueryHeaders")
	}
	ids, err := store.MatchingIDs(c, s, gq, []string{"FEAT-009", "FEAT-002"})
	require.NoError(t, err)
	assert.False(t, ids["FEAT-009"], "MatchingIDs")

	// A relation predicate must not match through a hidden edge: REQ-001's
	// only live inbound edge is from FEAT-002.
	viaHidden := store.GraphQuery{
		EntityType: "requirement",
		Faces:      store.InWorld(store.TrivialScope()),
		HasInbound: &store.RelationPredicate{Endpoints: []string{"FEAT-009"}, OfTypes: []string{"implements"}},
	}
	assert.Empty(t, entityIDs(t, s.GraphQuery(c, viaHidden)), "GraphQuery through a hidden edge")

	_, err = s.GetRelation(c, entity.RelationKey{From: "FEAT-009", Type: "implements", To: "REQ-001"})
	assert.ErrorIs(t, err, store.ErrNotFound, "GetRelation outgoing")
	_, err = s.GetRelation(c, entity.RelationKey{From: "FEAT-002", Type: "depends-on", To: "FEAT-009"})
	assert.ErrorIs(t, err, store.ErrNotFound, "GetRelation incoming")
	liveRel := []string{"FEAT-002--implements--REQ-001"}
	assert.Equal(t, liveRel, relationKeys(t, s.ListRelations(c, store.RelationQuery{})), "ListRelations")
	assert.Empty(t, relationKeys(t, s.ListRelations(c, store.RelationQuery{EntityID: "FEAT-009"})),
		"ListRelations by entity")
	assert.Empty(t, relationKeys(t, s.ListRelations(c, store.RelationQuery{EntityIDs: []string{"FEAT-009"}})),
		"ListRelations by entity batch")
	rpage, err := s.ListRelationsPage(c, store.RelationQuery{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, rpage.Items, 1, "ListRelationsPage")
	rn, err := s.CountRelations(c, store.RelationQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, rn, "CountRelations")

	if attachments {
		_, err = s.ListFamilyAttachments(c, "FEAT-009")
		assert.ErrorIs(t, err, store.ErrNotFound, "ListAttachments")
	}
}

// RunSoftDeleteTests is the conformance suite for [store.SoftDeleter]. It is
// the proof that a marked entity is hidden from every read path, since a
// read method that forgets the mark shows a deleted entity with no error.
// sf is optional; with it, the search index is checked too.
func RunSoftDeleteTests(t *testing.T, f Factory, sf SearchFactory, attachments bool) {
	t.Run("MarkHidesFromEveryRead", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)

		res, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)
		assert.Len(t, res.DeletedEntities, 2, "both faces are hidden")
		assert.Len(t, res.DeletedRelations, 2, "both incident edges are hidden")

		assertMarkedHidden(t, s, attachments)

		marked, err := sd.ListMarked(ctx())
		require.NoError(t, err)
		require.Len(t, marked, 1)
		assert.Equal(t, "FEAT-009", marked[0].ID)
		assert.Equal(t, "alice", marked[0].DeletedBy)
		assert.False(t, marked[0].DeletedAt.IsZero())
		require.Len(t, marked[0].Entities, 2)
		assert.True(t, marked[0].Entities[0].Face.IsImplicit(), "default face first")
		assert.Equal(t, "Marked", marked[0].Entities[0].GetString("title"))
	})

	t.Run("MarkedIDIsHeld", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		_, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)

		for _, e := range []*entity.Entity{
			entity.New("FEAT-009", "feature"),
			entity.New("feat-009", "feature"),
			func() *entity.Entity { e := entity.New("FEAT-009", "feature"); e.Face = "other"; return e }(),
		} {
			assert.ErrorIs(t, s.CreateEntity(ctx(), e), store.ErrConflict,
				"create %s@%s over a marked id", e.ID, e.Face)
		}
		_, err = s.RenameFamily(ctx(), "FEAT-002", "FEAT-009")
		assert.ErrorIs(t, err, store.ErrConflict, "rename onto a marked id")

		assert.ErrorIs(t, s.UpdateEntity(ctx(), entity.New("FEAT-009", "feature")), store.ErrNotFound)
		_, err = s.RenameFamily(ctx(), "FEAT-009", "FEAT-100")
		assert.ErrorIs(t, err, store.ErrNotFound)
		_, err = s.DeleteFamily(ctx(), "FEAT-009", true)
		assert.ErrorIs(t, err, store.ErrNotFound)
		_, err = sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		assert.ErrorIs(t, err, store.ErrNotFound, "a marked id cannot be marked twice")
		_, err = sd.MarkDeleted(ctx(), "NOPE-1", "alice")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("UnmarkRestoresEverything", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		before, err := s.GetEntity(ctx(), entity.Ref{ID: "FEAT-009"})
		require.NoError(t, err)

		_, err = sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)
		res, err := sd.Unmark(ctx(), "FEAT-009")
		require.NoError(t, err)
		assert.Len(t, res.DeletedEntities, 2)
		assert.Len(t, res.DeletedRelations, 2)

		after, err := s.GetEntity(ctx(), entity.Ref{ID: "FEAT-009"})
		require.NoError(t, err)
		assert.Equal(t, before.GetString("title"), after.GetString("title"))
		assert.Equal(t, before.Content, after.Content)
		draft, err := s.GetEntity(ctx(), entity.Ref{ID: "FEAT-009", Face: "draft"})
		require.NoError(t, err)
		assert.Equal(t, "Marked draft", draft.GetString("title"))

		rel, err := s.GetRelation(ctx(), entity.RelationKey{From: "FEAT-009", Type: "implements", To: "REQ-001"})
		require.NoError(t, err)
		assert.Equal(t, "edge", rel.Content, "a restored relation keeps its body")
		_, err = s.GetRelation(ctx(), entity.RelationKey{From: "FEAT-002", Type: "depends-on", To: "FEAT-009"})
		require.NoError(t, err)
		n, err := s.CountEntities(ctx(), store.EntityQuery{Faces: store.InWorld(store.TrivialScope())})
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		if attachments {
			infos, lerr := s.ListFamilyAttachments(ctx(), "FEAT-009")
			require.NoError(t, lerr)
			assert.Len(t, infos, 1, "attachments survive a mark")
		}
		marked, err := sd.ListMarked(ctx())
		require.NoError(t, err)
		assert.Empty(t, marked)

		_, err = sd.Unmark(ctx(), "FEAT-009")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("PurgeFreesTheID", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		_, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)

		res, err := sd.PurgeMarked(ctx(), "FEAT-009")
		require.NoError(t, err)
		assert.Len(t, res.DeletedEntities, 2)
		assert.Len(t, res.DeletedRelations, 2)
		marked, err := sd.ListMarked(ctx())
		require.NoError(t, err)
		assert.Empty(t, marked)
		_, err = sd.PurgeMarked(ctx(), "FEAT-009")
		assert.ErrorIs(t, err, store.ErrNotFound)
		_, err = sd.Unmark(ctx(), "FEAT-009")
		assert.ErrorIs(t, err, store.ErrNotFound)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("FEAT-009", "feature")),
			"a purged id is free again")
		assert.Empty(t, relationKeys(t, s.ListRelations(ctx(), store.RelationQuery{EntityID: "FEAT-009"})),
			"purged relations do not come back with a new entity of the same id")
		if attachments {
			infos, lerr := s.ListFamilyAttachments(ctx(), "FEAT-009")
			require.NoError(t, lerr)
			assert.Empty(t, infos, "purged attachments do not come back")
		}
	})

	t.Run("EdgeBetweenTwoMarkedEntities", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		_, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)
		_, err = sd.MarkDeleted(ctx(), "FEAT-002", "alice")
		require.NoError(t, err)

		_, err = sd.Unmark(ctx(), "FEAT-009")
		require.NoError(t, err)
		_, err = s.GetRelation(ctx(), entity.RelationKey{From: "FEAT-002", Type: "depends-on", To: "FEAT-009"})
		assert.ErrorIs(t, err, store.ErrNotFound, "an edge to a still-marked entity stays hidden")

		_, err = sd.Unmark(ctx(), "FEAT-002")
		require.NoError(t, err)
		_, err = s.GetRelation(ctx(), entity.RelationKey{From: "FEAT-002", Type: "depends-on", To: "FEAT-009"})
		assert.NoError(t, err, "the edge comes back with its last marked endpoint")
	})

	t.Run("PurgeDropsEdgesHeldByAnotherMark", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		// FEAT-002 first, so the shared edge is hidden with FEAT-002.
		_, err := sd.MarkDeleted(ctx(), "FEAT-002", "alice")
		require.NoError(t, err)
		_, err = sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)

		_, err = sd.PurgeMarked(ctx(), "FEAT-009")
		require.NoError(t, err)
		_, err = sd.Unmark(ctx(), "FEAT-002")
		require.NoError(t, err)
		_, err = s.GetRelation(ctx(), entity.RelationKey{From: "FEAT-002", Type: "depends-on", To: "FEAT-009"})
		assert.ErrorIs(t, err, store.ErrNotFound, "an edge to a purged entity must not come back")
	})

	// A hard delete or a rename of the other end drops the hidden edge, so a
	// restore cannot bring back an edge to an entity that is gone, or attach
	// it to a new entity that took the freed id.
	for _, tc := range []struct {
		name   string
		free   func(t *testing.T, s store.Store)
		reuse  string
		reType string
	}{
		{"HardDeleteOfOtherEndDropsHiddenEdge", func(t *testing.T, s store.Store) {
			t.Helper()
			_, err := s.DeleteFamily(ctx(), "REQ-001", true)
			require.NoError(t, err)
		}, "REQ-001", "requirement"},
		{"RenameOfOtherEndDropsHiddenEdge", func(t *testing.T, s store.Store) {
			t.Helper()
			_, err := s.RenameFamily(ctx(), "FEAT-002", "FEAT-003")
			require.NoError(t, err)
		}, "FEAT-002", "feature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := f(t)
			seedSoftDelete(t, s, attachments)
			sd := softDeleterOf(t, s)
			_, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
			require.NoError(t, err)

			tc.free(t, s)
			fresh := entity.New(tc.reuse, tc.reType)
			fresh.SetString("title", "Reused id")
			require.NoError(t, s.CreateEntity(ctx(), fresh), "the freed id can be taken again")

			res, err := sd.Unmark(ctx(), "FEAT-009")
			require.NoError(t, err)
			for _, r := range res.DeletedRelations {
				assert.NotContains(t, []string{r.From, r.To}, tc.reuse, "restored edge %s--%s--%s", r.From, r.Type, r.To)
			}
			for _, r := range []*entity.Relation{
				{From: "FEAT-009", Type: "implements", To: tc.reuse},
				{From: tc.reuse, Type: "depends-on", To: "FEAT-009"},
			} {
				_, err = s.GetRelation(ctx(), entity.RelationKey{From: r.From, Type: r.Type, To: r.To})
				assert.ErrorIs(t, err, store.ErrNotFound, "%s--%s--%s must not attach to the new %s",
					r.From, r.Type, r.To, tc.reuse)
			}
			for _, key := range relationKeys(t, s.ListRelations(ctx(), store.RelationQuery{EntityID: "FEAT-009"})) {
				assert.NotContains(t, key, tc.reuse)
			}
		})
	}

	t.Run("RevealIsNarrow", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		_, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)
		rc := store.WithRevealed(ctx(), "FEAT-009")

		// The two reads acl.StoreGraph makes see the marked entity's edges.
		_, err = s.GetRelation(rc, entity.RelationKey{From: "FEAT-009", Type: "implements", To: "REQ-001"})
		require.NoError(t, err, "GetRelation from the revealed entity")
		_, err = s.GetRelation(rc, entity.RelationKey{From: "FEAT-002", Type: "depends-on", To: "FEAT-009"})
		require.NoError(t, err, "GetRelation to the revealed entity")
		out := relationKeys(t, s.ListRelations(rc, store.RelationQuery{
			EntityID: "FEAT-009", Direction: store.DirectionOutgoing, Type: "implements",
		}))
		assert.Equal(t, []string{"FEAT-009--implements--REQ-001"}, out)

		// Nothing else does.
		_, err = s.GetEntity(rc, entity.Ref{ID: "FEAT-009"})
		assert.ErrorIs(t, err, store.ErrNotFound, "the entity itself stays hidden")
		assert.Equal(t, []string{"FEAT-002--implements--REQ-001"},
			relationKeys(t, s.ListRelations(rc, store.RelationQuery{})), "unscoped list")
		assert.Equal(t, []string{"FEAT-002--implements--REQ-001"},
			relationKeys(t, s.ListRelations(rc, store.RelationQuery{EntityID: "REQ-001"})),
			"a list scoped to another entity")
		rn, err := s.CountRelations(rc, store.RelationQuery{EntityID: "FEAT-009"})
		require.NoError(t, err)
		assert.Equal(t, 0, rn, "CountRelations")
		rpage, err := s.ListRelationsPage(rc, store.RelationQuery{EntityID: "FEAT-009", Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, rpage.Items, "ListRelationsPage")
		_, err = s.GetRelation(store.WithRevealed(ctx(), "REQ-001"), entity.RelationKey{From: "FEAT-009", Type: "implements", To: "REQ-001"})
		assert.ErrorIs(t, err, store.ErrNotFound, "revealing a live entity reveals nothing")

		// And an ordinary context never sees the marked edges.
		assertMarkedHidden(t, s, attachments)
	})

	// A restore of several marked entities reveals all of them, so an edge
	// held by one mark is visible from the other end too.
	t.Run("RevealSeveralMarks", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		// FEAT-002 first, so the shared edge is held by FEAT-002's mark.
		_, err := sd.MarkDeleted(ctx(), "FEAT-002", "alice")
		require.NoError(t, err)
		_, err = sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)
		shared := entity.RelationKey{From: "FEAT-002", Type: "depends-on", To: "FEAT-009"}
		incoming := store.RelationQuery{EntityID: "FEAT-009", Direction: store.DirectionIncoming}

		only := store.WithRevealed(ctx(), "FEAT-009")
		_, err = s.GetRelation(only, shared)
		assert.ErrorIs(t, err, store.ErrNotFound, "the edge is held by a mark that is not revealed")
		assert.Empty(t, relationKeys(t, s.ListRelations(only, incoming)))

		both := store.WithRevealed(ctx(), "FEAT-009", "FEAT-002")
		_, err = s.GetRelation(both, shared)
		require.NoError(t, err, "GetRelation with both marks revealed")
		assert.Equal(t, []string{"FEAT-002--depends-on--FEAT-009"},
			relationKeys(t, s.ListRelations(both, incoming)))
		assert.ElementsMatch(t, []string{"FEAT-002--depends-on--FEAT-009", "FEAT-009--implements--REQ-001"},
			relationKeys(t, s.ListRelations(both, store.RelationQuery{EntityID: "FEAT-009"})),
			"each hidden edge is listed once")
		assert.Empty(t, relationKeys(t, s.ListRelations(both, store.RelationQuery{EntityID: "REQ-001"})),
			"a list scoped to an entity that is not revealed")
	})

	t.Run("InsideTx", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		require.NoError(t, s.Tx(ctx(), func(tx store.Store) error {
			_, err := softDeleterOf(t, tx).MarkDeleted(ctx(), "FEAT-009", "alice")
			return err
		}))
		assertMarkedHidden(t, s, attachments)
		require.NoError(t, s.Tx(ctx(), func(tx store.Store) error {
			_, err := softDeleterOf(t, tx).Unmark(ctx(), "FEAT-009")
			return err
		}))
		_, err := s.GetEntity(ctx(), entity.Ref{ID: "FEAT-009"})
		require.NoError(t, err)
	})

	t.Run("Events", func(t *testing.T) {
		s := f(t)
		seedSoftDelete(t, s, attachments)
		sd := softDeleterOf(t, s)
		events, cancel := s.Subscribe(64)
		defer cancel()

		_, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
		require.NoError(t, err)
		waitForEvent(t, events, store.EventEntityDeleted, "FEAT-009")
		_, err = sd.Unmark(ctx(), "FEAT-009")
		require.NoError(t, err)
		waitForEvent(t, events, store.EventEntityCreated, "FEAT-009")
	})

	if sf != nil {
		t.Run("Search", func(t *testing.T) {
			s, searcher := sf(t)
			seedSoftDelete(t, s, attachments)
			sd := softDeleterOf(t, s)
			hits := func() []string {
				var ids []string
				for _, h := range collectHits(t, searcher.Search(ctx(), search.Query{Text: "Marked", World: store.TrivialScope()})) {
					ids = append(ids, h.ID)
				}
				return ids
			}
			require.Contains(t, hits(), "FEAT-009")
			_, err := sd.MarkDeleted(ctx(), "FEAT-009", "alice")
			require.NoError(t, err)
			assert.NotContains(t, hits(), "FEAT-009", "search must not find a marked entity")
			_, err = sd.Unmark(ctx(), "FEAT-009")
			require.NoError(t, err)
			assert.Contains(t, hits(), "FEAT-009", "search finds it again after Unmark")
		})
	}
}

// waitForEvent drains events until one with op for id arrives.
func waitForEvent(t *testing.T, events <-chan store.Event, op store.EventOp, id string) {
	t.Helper()
	c, cancel := context.WithTimeout(ctx(), 5*time.Second)
	defer cancel()
	for {
		select {
		case ev := <-events:
			if ev.Op == op && ev.EntityID == id {
				return
			}
		case <-c.Done():
			t.Fatalf("no event %v for %s: %v", op, id, errors.New("timed out"))
		}
	}
}
