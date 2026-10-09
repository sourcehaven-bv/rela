package datamigration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// faceMoveProbe records whether the writes a FACE MOVE makes (the create at the
// destination, the delete of the source, and the carry of its edges) arrived
// through a transaction view.
//
// A failure between them leaves the row duplicated rather than moved. Only a transaction makes the pair atomic, so
// "did these writes go through Tx" is the property worth pinning — and unlike
// rollback it is checkable on memstore, which has none.
//
// Named for what it measures, NOT `txProbe`: it overrides the four of
// store.Store's write methods a move uses, and the rest promote from the
// embedded interface invisibly. A future step that calls UpdateEntity would get a green result
// this type never examined. The `writes` count is asserted alongside
// `bareTxns` so an unobserved write cannot hide behind an expected total.
type faceMoveProbe struct {
	store.Store
	inTx     bool
	writes   int
	bareTxns int // writes that reached the store OUTSIDE any Tx
	txCalls  int // transactions opened
}

func (p *faceMoveProbe) Tx(ctx context.Context, fn func(store.Store) error) error {
	p.txCalls++
	return p.Store.Tx(ctx, func(view store.Store) error {
		inner := &faceMoveProbe{Store: view, inTx: true}
		err := fn(inner)
		p.writes += inner.writes
		p.bareTxns += inner.bareTxns
		return err
	})
}

func (p *faceMoveProbe) CreateEntity(ctx context.Context, e *entity.Entity) error {
	p.note()
	return p.Store.CreateEntity(ctx, e)
}

func (p *faceMoveProbe) DeleteFace(ctx context.Context, ref entity.Ref) (*store.DeleteResult, error) {
	p.note()
	return p.Store.DeleteFace(ctx, ref)
}

func (p *faceMoveProbe) CreateRelation(
	ctx context.Context, k entity.RelationKey, data *store.RelationData,
) (*entity.Relation, error) {
	p.note()
	return p.Store.CreateRelation(ctx, k, data)
}

func (p *faceMoveProbe) DeleteRelation(ctx context.Context, k entity.RelationKey) error {
	p.note()
	return p.Store.DeleteRelation(ctx, k)
}

func (p *faceMoveProbe) note() {
	p.writes++
	if !p.inTx {
		p.bareTxns++
	}
}

// Every row a migrate_face step moves must be written inside a transaction.
// Before BUG-TOX8U4 the apply loop wrote straight to the outer store, so an
// error part-way left the family split across the bare coordinate and a face —
// a shape `rela analyze states` now reports as stranded (BUG-UA3BK3).
func TestMigrateFace_MovesRunInsideATransaction(t *testing.T) {
	probe := &faceMoveProbe{Store: seedStore(t)}
	r := newTestRunner(t, Deps{Store: probe, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "20260919143022-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))

	// TSK-1 owns one content edge, which the move carries and removes.
	seedReviewEdge(t, probe.Store, "PER-1")
	if _, err := r.Run(t.Context(), []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Three tasks move, each a create plus a delete, and one edge is carried:
	// a create plus a delete.
	if probe.writes != 8 {
		t.Errorf("writes = %d, want 8 (3 moves x create+delete, 1 edge x create+delete)", probe.writes)
	}
	if probe.bareTxns != 0 {
		t.Errorf("%d write(s) reached the store outside a transaction; a move's create and "+
			"delete must be atomic or a failure between them duplicates the row", probe.bareTxns)
	}
}

// rename_face carries the identical create+delete pair and the identical
// hazard, so it gets the identical guarantee. Fixing one and not the other
// would have been arbitrary: the two steps differ in which coordinate they
// move a row FROM, not in what a half-applied move leaves behind.
func TestRenameFace_MovesRunInsideATransaction(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	// Put the tasks on a named face so there is something to rename.
	for _, id := range []string{"TSK-1", "TSK-2", "TSK-3"} {
		e, err := st.GetEntity(ctx, entity.Ref{ID: id})
		if err != nil {
			t.Fatalf("seed read %s: %v", id, err)
		}
		moved := *e
		moved.Face = entity.Face("nl")
		if err := st.CreateEntity(ctx, &moved); err != nil {
			t.Fatalf("seed %s@nl: %v", id, err)
		}
	}

	probe := &faceMoveProbe{Store: st}
	r := newTestRunner(t, Deps{Store: probe, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "20260919143022-rename.yaml", mustFileYAML(t,
		facedMeta("nl", "nl-be"), facedMeta("nl-be"),
		"  - rename_face: {entity: task, from: nl, to: nl-be}\n"))

	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if probe.bareTxns != 0 {
		t.Errorf("%d rename write(s) reached the store outside a transaction", probe.bareTxns)
	}
	if probe.writes == 0 {
		t.Error("the probe saw no writes at all; the test is not exercising the apply path")
	}
}

// A move must carry the row's content-scoped edges to the new face and leave
// its identity-scoped edges where they are.
//
// The create at the destination copies content, not edges, so without the
// carry a move silently destroys every relation the row owned (BUG-TOX8U4).
// An identity-scoped edge belongs to the entity rather than to the implicit
// row: re-tailing it onto a face would let a later delete of that face take
// it, and the manager refuses to write an identity edge with a face.
func TestMigrateFace_CarriesContentEdgesAndKeepsIdentityEdges(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	review := seedReviewEdge(t, st, "PER-1")

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "20260919143022-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// TSK-1's status is `open`, which the migration maps to `draft`.
	assertEdges(t, st, map[entity.RelationKey]map[string]any{
		{From: "TSK-1", Type: "assigned-to", To: "PER-1"}:                  {"weight": "high"},
		{From: "TSK-1", FromFace: "draft", Type: review.Type, To: "PER-1"}: {"weight": "low"},
	})
}

// The batch boundary is arithmetic, and arithmetic is what a refactor breaks.
//
// updateBatchSize exists because one transaction over every move stalls every
// other writer on pg, so the count of transactions is the property the comment
// claims and nothing else asserts. A slice bound that silently became
// off-by-one would still pass every other test here.
func TestMigrateFace_BatchesAtTheDocumentedBoundary(t *testing.T) {
	for _, tc := range []struct{ moves, wantTx int }{
		{0, 0},
		{1, 1},
		{updateBatchSize, 1},
		{updateBatchSize + 1, 2},
		{updateBatchSize * 2, 2},
	} {
		probe := &faceMoveProbe{Store: memstore.New()}
		moves := make([]faceMove, 0, tc.moves)
		for i := range tc.moves {
			e := entity.New(fmt.Sprintf("TSK-%d", i), "task")
			if err := probe.Store.CreateEntity(t.Context(), e); err != nil {
				t.Fatalf("seed: %v", err)
			}
			moves = append(moves, faceMove{e: e, to: "draft"})
		}
		if err := applyMoves(t.Context(), probe, nil, moves, relationScopes{known: true}); err != nil {
			t.Fatalf("%d moves: %v", tc.moves, err)
		}
		if probe.txCalls != tc.wantTx {
			t.Errorf("%d moves: %d transaction(s), want %d", tc.moves, probe.txCalls, tc.wantTx)
		}
	}
}

// errInjected is the failure carryFault injects.
var errInjected = errors.New("injected carry failure")

// carryFault fails the failOn-th CreateRelation, counting through every
// transaction view it hands out.
type carryFault struct {
	store.Store
	failOn int
	calls  *int
}

func (f *carryFault) Tx(ctx context.Context, fn func(store.Store) error) error {
	return f.Store.Tx(ctx, func(view store.Store) error {
		return fn(&carryFault{Store: view, failOn: f.failOn, calls: f.calls})
	})
}

func (f *carryFault) CreateRelation(
	ctx context.Context, k entity.RelationKey, data *store.RelationData,
) (*entity.Relation, error) {
	*f.calls++
	if *f.calls == f.failOn {
		return nil, errInjected
	}
	return f.Store.CreateRelation(ctx, k, data)
}

// seedReviewEdge links TSK-1 --reviewed-by--> to from the zero tail. The
// relation is content-scoped, so a face move carries it.
func seedReviewEdge(t *testing.T, st store.Store, to string) entity.RelationKey {
	t.Helper()
	k := entity.RelationKey{From: "TSK-1", Type: "reviewed-by", To: to}
	if _, err := st.CreateRelation(t.Context(), k,
		&store.RelationData{Properties: map[string]any{"weight": "low"}}); err != nil {
		t.Fatalf("seed %s: %v", k.Type, err)
	}
	return k
}

// assertEdges checks that st holds exactly the edges in want, with the given
// properties.
func assertEdges(t *testing.T, st store.Store, want map[entity.RelationKey]map[string]any) {
	t.Helper()
	got := map[entity.RelationKey]map[string]any{}
	for rel, err := range st.ListRelations(t.Context(), store.RelationQuery{}) {
		if err != nil {
			t.Fatalf("list relations: %v", err)
		}
		got[rel.Identity()] = rel.Properties
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("edges = %v, want %v", got, want)
	}
}

// A move that fails while carrying edges must leave a re-run able to finish.
//
// fs/mem have no rollback, so whatever the failed run wrote stays written.
// The source row used to be deleted before its edges were carried, so an
// edge not yet carried existed nowhere and a re-run, which finds rows by
// scanning, had no source row left to carry it from (GitHub #1628).
func TestMigrateFace_RerunRecoversAFailedEdgeCarry(t *testing.T) {
	for _, b := range []struct {
		name string
		open func(*testing.T) store.Store
	}{
		{"memstore", func(*testing.T) store.Store { return memstore.New() }},
		{"fsstore", openFSStore},
	} {
		t.Run(b.name, func(t *testing.T) { rerunRecoversAFailedEdgeCarry(t, seedInto(t, b.open(t))) })
	}
}

// openFSStore opens an fsstore over an in-memory filesystem. Its Tx is a
// write mutex with no rollback, which is the case the recovery exists for.
func openFSStore(t *testing.T) store.Store {
	t.Helper()
	mfs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(mfs, "/")
	if err != nil {
		t.Fatalf("rooted fs: %v", err)
	}
	st, err := fsstore.New(fsstore.Config{
		FS: mfs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations",
		AttachmentsKey: "attachments", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"task":   {Plural: "tasks"},
			"person": {Plural: "persons"},
		},
	})
	if err != nil {
		t.Fatalf("fsstore.New: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func rerunRecoversAFailedEdgeCarry(t *testing.T, st store.Store) {
	t.Helper()
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "PER-2", Type: "person", Properties: map[string]any{"name": "Grace"},
	}); err != nil {
		t.Fatalf("seed PER-2: %v", err)
	}
	// Two content edges, so the failure lands after one has been carried.
	first, second := seedReviewEdge(t, st, "PER-1"), seedReviewEdge(t, st, "PER-2")
	f := mustParse(t, "20260919143022-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	state := newFakeKV()

	calls := 0
	faulty := &carryFault{Store: st, failOn: 2, calls: &calls}
	r := newTestRunner(t, Deps{Store: faulty, State: state, Audit: audit.NewMemory()})
	if _, err := r.Run(ctx, []*File{f}, true); !errors.Is(err, errInjected) {
		t.Fatalf("Run: want the injected carry failure, got %v", err)
	}
	// The regression itself: after the failure the source row and the edge
	// the failed run did not carry must both still exist.
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1"}); err != nil {
		t.Errorf("the source row must survive a failed carry: %v", err)
	}
	if _, err := st.GetRelation(ctx, second); err != nil {
		t.Errorf("the uncarried edge must survive a failed carry: %v", err)
	}

	r = newTestRunner(t, Deps{Store: st, State: state, Audit: audit.NewMemory()})
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1"}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the source row must be gone after the re-run: %v", err)
	}
	draft := func(k entity.RelationKey) entity.RelationKey { k.FromFace = "draft"; return k }
	assertEdges(t, st, map[entity.RelationKey]map[string]any{
		{From: "TSK-1", Type: "assigned-to", To: "PER-1"}: {"weight": "high"},
		draft(first):  {"weight": "low"},
		draft(second): {"weight": "low"},
	})
}

// An edge already at the destination with different data is a distinct edge.
// Carrying over it would let the move destroy one of the two, so the move is
// refused before its first write.
func TestMigrateFace_RefusesToCarryOverADifferentEdge(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	src := seedReviewEdge(t, st, "PER-1")
	draft := entity.New("TSK-1", "task")
	draft.Face = "draft"
	draft.Properties = getEntity(t, st, "TSK-1").Properties
	if err := st.CreateEntity(ctx, draft); err != nil {
		t.Fatalf("seed TSK-1@draft: %v", err)
	}
	clash := src
	clash.FromFace = draft.Face
	if _, err := st.CreateRelation(ctx, clash,
		&store.RelationData{Properties: map[string]any{"weight": "high"}}); err != nil {
		t.Fatalf("seed clashing edge: %v", err)
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "20260919143022-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	if _, err := r.Run(ctx, []*File{f}, true); !errors.Is(err, errEdgeClash) {
		t.Fatalf("Run: want errEdgeClash, got %v", err)
	}
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1"}); err != nil {
		t.Errorf("the source row must survive a refused move: %v", err)
	}
	assertEdges(t, st, map[entity.RelationKey]map[string]any{
		{From: "TSK-1", Type: "assigned-to", To: "PER-1"}: {"weight": "high"},
		src:   {"weight": "low"},
		clash: {"weight": "high"},
	})
}

// rename_face moves a row between two named faces, so every edge on the old
// tail is the row's own and moves with it.
func TestRenameFace_CarriesTheFaceEdges(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	nl := getEntity(t, st, "TSK-1")
	nl.Face = "nl"
	if err := st.CreateEntity(ctx, nl); err != nil {
		t.Fatalf("seed TSK-1@nl: %v", err)
	}
	edge := entity.RelationKey{From: "TSK-1", FromFace: "nl", Type: "reviewed-by", To: "PER-1"}
	if _, err := st.CreateRelation(ctx, edge,
		&store.RelationData{Properties: map[string]any{"weight": "low"}}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "20260919143022-rename.yaml", mustFileYAML(t,
		facedMeta("nl", "nl-be"), facedMeta("nl-be"),
		"  - rename_face: {entity: task, from: nl, to: nl-be}\n"))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}
	moved := edge
	moved.FromFace = "nl-be"
	assertEdges(t, st, map[entity.RelationKey]map[string]any{
		{From: "TSK-1", Type: "assigned-to", To: "PER-1"}: {"weight": "high"},
		moved: {"weight": "low"},
	})
}

// rename_face moves a row between two named faces. An identity-scoped edge
// on the old face's tail is one an earlier move put there by mistake, so the
// move puts it back on the zero tail rather than onto the new face.
func TestRenameFace_ReturnsAnIdentityEdgeToTheEntity(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	nl := getEntity(t, st, "TSK-1")
	nl.Face = "nl"
	if err := st.CreateEntity(ctx, nl); err != nil {
		t.Fatalf("seed TSK-1@nl: %v", err)
	}
	stray := entity.RelationKey{From: "TSK-1", FromFace: "nl", Type: "assigned-to", To: "PER-1"}
	if err := st.DeleteRelation(ctx, entity.RelationKey{From: "TSK-1", Type: "assigned-to", To: "PER-1"}); err != nil {
		t.Fatalf("clear the seeded edge: %v", err)
	}
	if _, err := st.CreateRelation(ctx, stray,
		&store.RelationData{Properties: map[string]any{"weight": "high"}}); err != nil {
		t.Fatalf("seed stray edge: %v", err)
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "20260919143022-rename.yaml", mustFileYAML(t,
		facedMeta("nl", "nl-be"), facedMeta("nl-be"),
		"  - rename_face: {entity: task, from: nl, to: nl-be}\n"))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertEdges(t, st, map[entity.RelationKey]map[string]any{
		{From: "TSK-1", Type: "assigned-to", To: "PER-1"}: {"weight": "high"},
	})
}

// A migration file whose projections predate relation scopes cannot say
// which edges belong to the row, so a move of a row that has edges is
// refused rather than guessed. A row without edges still moves.
func TestMigrateFace_RefusesEdgesWhenScopesAreUnknown(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	fp, tp := metaV1().ShapeProjection(), facedV1().ShapeProjection()
	fp.RelationScopes, tp.RelationScopes = false, false
	fromYAML, err := marshalProjectionYAML("from_projection", fp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	toYAML, err := marshalProjectionYAML("to_projection", tp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	f := mustParse(t, "20260919143022-faces.yaml",
		[]byte("description: test\nsteps:\n"+migrateTaskFaces+fromYAML+toYAML))

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	if _, err := r.Run(ctx, []*File{f}, true); !errors.Is(err, errScopesUnknown) {
		t.Fatalf("Run: want errScopesUnknown, got %v", err)
	}
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "TSK-1"}); err != nil {
		t.Errorf("TSK-1 has an edge and must not move: %v", err)
	}
}

// moveCapture records the delete versions a move writes, by address.
type moveCapture struct {
	entities, relations []string
}

func (c *moveCapture) WriteVersion(_ context.Context, in store.VersionInput) error {
	c.entities = append(c.entities, entity.FormatStateRef(in.EntityID, in.Face))
	return nil
}

func (c *moveCapture) WriteRelationVersion(_ context.Context, in store.RelationVersionInput) error {
	c.relations = append(c.relations, entity.FormatStateRef(in.Key.From, in.Key.FromFace)+
		"--"+in.Key.Type+"--"+in.Key.To)
	return nil
}

// A move deletes the source row and the old key of every edge it moves, and
// the sweep cannot reconstruct either, so both are captured before the move.
// An identity edge that stays on the zero tail keeps its key and is not.
func TestMigrateFace_CapturesWhatTheMoveDeletes(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	seedReviewEdge(t, st, "PER-1")
	capture := &moveCapture{}
	r := newTestRunner(t, Deps{Store: st, Meta: facedV1(), Versions: capture})
	f := mustParse(t, "20260919143022-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if want := []string{"TSK-1", "TSK-2", "TSK-3"}; !reflect.DeepEqual(capture.entities, want) {
		t.Errorf("entity captures = %v, want %v", capture.entities, want)
	}
	if want := []string{"TSK-1--reviewed-by--PER-1"}; !reflect.DeepEqual(capture.relations, want) {
		t.Errorf("relation captures = %v, want %v", capture.relations, want)
	}
}
