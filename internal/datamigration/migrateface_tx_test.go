package datamigration

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// faceMoveProbe records whether the two writes a FACE MOVE makes — the create
// at the destination and the delete of the source — arrived through a
// transaction view.
//
// A move is exactly that pair, and a failure between them leaves the row
// duplicated rather than moved. Only a transaction makes the pair atomic, so
// "did these writes go through Tx" is the property worth pinning — and unlike
// rollback it is checkable on memstore, which has none.
//
// Named for what it measures, NOT `txProbe`: it overrides two of store.Store's
// eleven write methods, and the rest promote from the embedded interface
// invisibly. A future step that calls UpdateEntity would get a green result
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

func (p *faceMoveProbe) DeleteEntityState(
	ctx context.Context, id string, f entity.Face,
) (*store.DeleteResult, error) {
	p.note()
	return p.Store.DeleteEntityState(ctx, id, f)
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

	if _, err := r.Run(t.Context(), []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Three tasks move, each a create plus a delete.
	if probe.writes != 6 {
		t.Errorf("writes = %d, want 6 (3 moves x create+delete)", probe.writes)
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
		e, err := st.GetEntity(ctx, id)
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
		facedMeta("nl", "nl-BE"), facedMeta("nl-BE"),
		"  - rename_face: {entity: task, from: nl, to: nl-BE}\n"))

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

// A move must carry the row's OUTGOING edges to the new face.
//
// Deleting a face takes its outgoing edges with it (the DeleteEntityState
// contract: they were written against that face and nothing else can own
// them), and the create at the destination copies content, not edges. Before
// this was handled, adopting faces on a populated type silently destroyed
// every relation those rows owned — no error, no history row, and `rela
// analyze` reports a graph that is merely smaller, not broken.
func TestMigrateFace_CarriesOutgoingEdgesToTheNewFace(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()

	// seedStore links TSK-1 --assigned-to--> PER-1 from the bare face.
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "20260919143022-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got []*entity.Relation
	for rel, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			t.Fatalf("list relations: %v", err)
		}
		got = append(got, rel)
	}
	if len(got) != 1 {
		t.Fatalf("the edge must survive the move, once: got %d relation(s): %+v", len(got), got)
	}
	// TSK-1's status is `open`, which the migration maps to `draft`, so the
	// edge follows its tail onto that face rather than staying behind.
	if got[0].From != "TSK-1" || got[0].To != "PER-1" || string(got[0].FromFace) != "draft" {
		t.Errorf("edge should be re-tailed on the destination face: %s@%q --%s--> %s",
			got[0].From, got[0].FromFace, got[0].Type, got[0].To)
	}
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
		if err := applyMoves(t.Context(), &Exec{Store: probe, Apply: true}, moves); err != nil {
			t.Fatalf("%d moves: %v", tc.moves, err)
		}
		if probe.txCalls != tc.wantTx {
			t.Errorf("%d moves: %d transaction(s), want %d", tc.moves, probe.txCalls, tc.wantTx)
		}
	}
}
