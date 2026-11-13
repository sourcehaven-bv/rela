package entitymanager_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// batchNote is a batch entity create of a note titled title.
func batchNote(title string) entitymanager.BatchEntity {
	e := entity.New("", "note")
	e.SetString("title", title)
	return entitymanager.BatchEntity{Entity: e}
}

// local is a batch end naming the batch entity at i.
func local(i int) entitymanager.BatchEnd { return entitymanager.BatchEnd{Index: i} }

// existing is a batch end naming the stored entity id.
func existing(id string) entitymanager.BatchEnd { return entitymanager.BatchEnd{ID: id} }

// successOps lists the ops of the success records in records, in order.
func successOps(records []audit.Record) []string {
	var ops []string
	for _, r := range records {
		if r.Op != audit.OpDeniedWrite {
			ops = append(ops, r.Op)
		}
	}
	return ops
}

// countNotes counts the notes in st.
func countNotes(t *testing.T, st store.Store) int {
	t.Helper()
	n, err := st.CountEntities(context.Background(), store.EntityQuery{Type: "note", Faces: store.AllFaces()})
	require.NoError(t, err)
	return n
}

// drainEvents returns the events that arrive on ch within wait.
func drainEvents(ch <-chan store.Event, wait time.Duration) []store.Event {
	var out []store.Event
	timeout := time.After(wait)
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-timeout:
			return out
		}
	}
}

// TestCreateBatch_CommitsTogether pins that a batch writes every entity and
// relation, that an edge between two batch entities resolves to their
// generated ids, and that every audit record precedes the follow-ups.
func TestCreateBatch_CommitsTogether(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, sink := openLinkedBackend(t, backend)
			target := mustCreateNote(t, mgr, "target")
			before := len(sink.Records())
			res, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
				Entities: []entitymanager.BatchEntity{batchNote("a"), batchNote("b"), batchNote("c")},
				Relations: []entitymanager.BatchRelation{
					{From: local(0), Type: "links", To: local(1)},
					{From: local(1), Type: "links", To: existing(target.ID)},
					{From: existing(target.ID), Type: "links", To: local(2)},
				},
			})
			require.NoError(t, err)
			require.Len(t, res.Entities, 3)
			ids := map[string]bool{target.ID: true}
			for i, r := range res.Entities {
				require.NoError(t, res.FollowUpErrs[i])
				require.False(t, ids[r.Entity.ID], "generated ids are distinct: %s", r.Entity.ID)
				ids[r.Entity.ID] = true
			}
			a, b, c := res.Entities[0].Entity.ID, res.Entities[1].Entity.ID, res.Entities[2].Entity.ID
			for _, key := range []entity.RelationKey{
				{From: a, Type: "links", To: b},
				{From: b, Type: "links", To: target.ID},
				{From: target.ID, Type: "links", To: c},
			} {
				_, err := st.GetRelation(context.Background(), key)
				require.NoError(t, err, "edge %v", key)
			}
			require.Equal(t, res.Relations[0].From, a)
			require.Equal(t, res.Relations[0].To, b)
			require.Equal(t, []string{
				audit.OpCreateEntity, audit.OpCreateEntity, audit.OpCreateEntity,
				audit.OpCreateRelation, audit.OpCreateRelation, audit.OpCreateRelation,
			}, successOps(sink.Records()[before:]))
		})
	}
}

// TestCreateBatch_MidBatchFailureRollsBack pins the atomicity: a failure
// in the last operation leaves, on a store that rolls back, no rows, no
// audit records and no store events. The filesystem and memory stores keep
// what they wrote, audit it, and report it.
func TestCreateBatch_MidBatchFailureRollsBack(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, sink := openLinkedBackend(t, backend)
			target := mustCreateNote(t, mgr, "target")
			events, cancel := st.Subscribe(64)
			defer cancel()
			drainEvents(events, 100*time.Millisecond)
			before := len(sink.Records())
			notesBefore := countNotes(t, st)
			// The same edge twice: both pass the checks, and the second
			// write conflicts with the first inside the transaction.
			_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
				Entities: []entitymanager.BatchEntity{batchNote("a"), batchNote("b")},
				Relations: []entitymanager.BatchRelation{
					{From: local(0), Type: "links", To: existing(target.ID)},
					{From: local(1), Type: "links", To: local(0)},
					{From: local(0), Type: "links", To: existing(target.ID)},
				},
			})
			require.ErrorIs(t, err, entitymanager.ErrRelationAlreadyExists)
			op, ok := errors.AsType[*entitymanager.BatchOpError](err)
			require.True(t, ok, "the failing operation is named: %v", err)
			require.Equal(t, entitymanager.BatchOpRelation, op.Kind)
			require.Equal(t, 2, op.Index)
			if rollsBack(backend) {
				require.Equal(t, notesBefore, countNotes(t, st), "no entity stays")
				require.Empty(t, sink.Records()[before:], "nothing is audited for a rolled-back batch")
				require.Empty(t, drainEvents(events, 200*time.Millisecond), "no event is delivered")
				_, kept := errors.AsType[*entitymanager.BatchKeptError](err)
				require.False(t, kept)
				return
			}
			kept, isKept := errors.AsType[*entitymanager.BatchKeptError](err)
			require.True(t, isKept, "kept writes are reported: %v", err)
			require.Len(t, kept.Entities, 2)
			require.Len(t, kept.Relations, 2)
			require.Equal(t, notesBefore+2, countNotes(t, st))
			require.Equal(t, []string{
				audit.OpCreateEntity, audit.OpCreateEntity, audit.OpCreateRelation, audit.OpCreateRelation,
			}, successOps(sink.Records()[before:]), "a kept row is audited")
		})
	}
}

// TestCreateBatch_RollbackThenCommitDeliversEvents is the control for the
// rollback test's "no event": the same subscription does see the events of
// a batch that commits.
func TestCreateBatch_RollbackThenCommitDeliversEvents(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, _ := openLinkedBackend(t, backend)
			events, cancel := st.Subscribe(64)
			defer cancel()
			_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
				Entities:  []entitymanager.BatchEntity{batchNote("a"), batchNote("b")},
				Relations: []entitymanager.BatchRelation{{From: local(0), Type: "links", To: local(1)}},
			})
			require.NoError(t, err)
			require.Eventually(t, func() bool {
				return len(drainEvents(events, 50*time.Millisecond)) > 0
			}, 5*time.Second, 10*time.Millisecond)
		})
	}
}

// TestCreateBatch_RefusalWritesNothing pins that a refused check, here an
// edge whose source type the relation does not allow, fails before
// anything is written, on every store, and names the operation.
func TestCreateBatch_RefusalWritesNothing(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, sink := openLinkedBackend(t, backend)
			before := len(sink.Records())
			_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
				Entities: []entitymanager.BatchEntity{batchNote("a"), batchNote("b")},
				Relations: []entitymanager.BatchRelation{
					{From: local(0), Type: "links", To: local(1)},
					{From: local(0), Type: "lists", To: local(1)},
				},
			})
			invalid, ok := errors.AsType[*entitymanager.InvalidRelationError](err)
			require.True(t, ok, "a type-disallowed edge is refused: %v", err)
			require.Equal(t, metamodel.CodeSourceTypeNotAllowed, invalid.TypeNotAllowedCode())
			op, ok := errors.AsType[*entitymanager.BatchOpError](err)
			require.True(t, ok)
			require.Equal(t, entitymanager.BatchOpRelation, op.Kind)
			require.Equal(t, 1, op.Index)
			require.Equal(t, entitymanager.CreateErrInvalid, entitymanager.ClassifyCreateError(err))
			require.Zero(t, countNotes(t, st))
			require.Empty(t, successOps(sink.Records()[before:]))
		})
	}
}

// TestCreateBatch_MissingEndpoint pins that an existing end that does not
// exist is refused as a missing entity before anything is written.
func TestCreateBatch_MissingEndpoint(t *testing.T) {
	_, mgr, _ := openLinkedBackend(t, concBackends[0])
	for end, rel := range map[string]entitymanager.BatchRelation{
		entitymanager.BatchEndTo:   {From: local(0), Type: "links", To: existing("NOTE-999")},
		entitymanager.BatchEndFrom: {From: existing("NOTE-999"), Type: "links", To: local(0)},
	} {
		_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
			Entities:  []entitymanager.BatchEntity{batchNote("a")},
			Relations: []entitymanager.BatchRelation{rel},
		})
		require.ErrorIs(t, err, entitymanager.ErrEntityNotFound)
		op, ok := errors.AsType[*entitymanager.BatchOpError](err)
		require.True(t, ok)
		require.Equal(t, end, op.End)
	}
}

// TestCreateBatch_Shape pins the shape refusals: nothing is checked,
// audited or written for a batch the engine cannot run.
func TestCreateBatch_Shape(t *testing.T) {
	_, mgr, sink := openLinkedBackend(t, concBackends[0])
	tooMany := make([]entitymanager.BatchEntity, entitymanager.MaxBatchOperations+1)
	for i := range tooMany {
		tooMany[i] = batchNote("n")
	}
	for name, b := range map[string]entitymanager.Batch{
		"empty":      {},
		"too many":   {Entities: tooMany},
		"nil entity": {Entities: []entitymanager.BatchEntity{{}}},
		"index out of range": {
			Entities:  []entitymanager.BatchEntity{batchNote("a")},
			Relations: []entitymanager.BatchRelation{{From: local(0), Type: "links", To: local(1)}},
		},
		"local tail with a face": {
			Entities: []entitymanager.BatchEntity{batchNote("a"), batchNote("b")},
			Relations: []entitymanager.BatchRelation{{
				From: entitymanager.BatchEnd{Index: 0, Face: "x"}, Type: "links", To: local(1),
			}},
		},
		"head with a face": {
			Entities: []entitymanager.BatchEntity{batchNote("a"), batchNote("b")},
			Relations: []entitymanager.BatchRelation{{
				From: local(0), Type: "links", To: entitymanager.BatchEnd{Index: 1, Face: "x"},
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := entitymanager.CreateBatch(context.Background(), mgr, b)
			require.ErrorIs(t, err, entitymanager.ErrBatchInvalid)
			require.Empty(t, sink.Records())
		})
	}
}

// TestCreateBatch_ExactlyMaxOperations pins that the limit is inclusive.
func TestCreateBatch_ExactlyMaxOperations(t *testing.T) {
	st, mgr, _ := openLinkedBackend(t, concBackends[0])
	entities := make([]entitymanager.BatchEntity, entitymanager.MaxBatchOperations)
	for i := range entities {
		entities[i] = batchNote("n")
	}
	_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{Entities: entities})
	require.NoError(t, err)
	require.Equal(t, entitymanager.MaxBatchOperations, countNotes(t, st))
}

// txHookStore runs before ahead of each outer Tx and after once it
// returns, on the store it wraps, to place another writer's work in the
// gaps around a batch's transaction.
type txHookStore struct {
	store.Store
	before func(ctx context.Context)
	after  func(ctx context.Context, err error)
}

func (s *txHookStore) Unwrap() store.Store { return s.Store }

func (s *txHookStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	if s.before != nil {
		s.before(ctx)
	}
	err := s.Store.Tx(ctx, fn)
	if s.after != nil {
		s.after(ctx, err)
	}
	return err
}

// managerOver is openLinkedBackend's manager over st.
func managerOver(t *testing.T, st store.Store) (*entitymanager.Manager, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(concurrencyMetamodelYAML))
	require.NoError(t, err)
	sink := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: sink, ACL: acl.NopACL{},
		Transitions: transitions.NoTransitions(), FieldGate: entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	require.NoError(t, err)
	return mgr, sink
}

// TestCreateBatch_UniqueWithinBatch pins that a unique value used twice in
// one batch is refused inside the transaction, which counts the batch's
// own earlier rows; a store that rolls back keeps neither.
func TestCreateBatch_UniqueWithinBatch(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, _ := openLinkedBackend(t, backend)
			person := func() entitymanager.BatchEntity {
				e := entity.New("", "person")
				e.SetString("email", "a@example.com")
				return entitymanager.BatchEntity{Entity: e}
			}
			_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
				Entities: []entitymanager.BatchEntity{person(), person()},
			})
			op, ok := errors.AsType[*entitymanager.BatchOpError](err)
			require.True(t, ok, "the second create is refused: %v", err)
			require.Equal(t, 1, op.Index)
			if rollsBack(backend) {
				n, cErr := st.CountEntities(context.Background(),
					store.EntityQuery{Type: "person", Faces: store.AllFaces()})
				require.NoError(t, cErr)
				require.Zero(t, n)
			}
		})
	}
}

// TestCreateBatch_ConcurrentBatchesMintDistinctIDs pins that generated ids
// are minted under the store's write lock: batches running at once never
// share an id.
func TestCreateBatch_ConcurrentBatchesMintDistinctIDs(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, _ := openLinkedBackend(t, backend)
			const batches, per = 4, 5
			errs := make(chan error, batches)
			for range batches {
				go func() {
					entities := make([]entitymanager.BatchEntity, per)
					for i := range entities {
						entities[i] = batchNote("n")
					}
					_, err := entitymanager.CreateBatch(context.Background(), mgr,
						entitymanager.Batch{Entities: entities})
					errs <- err
				}()
			}
			for range batches {
				require.NoError(t, <-errs)
			}
			require.Equal(t, batches*per, countNotes(t, st))
		})
	}
}

// TestCreateBatch_EndDeletedBeforeTx pins that an existing end deleted
// after the checks but before the transaction is refused inside it, as a
// missing end, with nothing written.
func TestCreateBatch_EndDeletedBeforeTx(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, seedMgr, _ := openLinkedBackend(t, backend)
			target := mustCreateNote(t, seedMgr, "target")
			hooked := &txHookStore{Store: st}
			hooked.before = func(ctx context.Context) {
				_, dErr := st.DeleteFamily(ctx, target.ID, true)
				require.NoError(t, dErr)
			}
			mgr, sink := managerOver(t, hooked)
			_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
				Entities:  []entitymanager.BatchEntity{batchNote("a")},
				Relations: []entitymanager.BatchRelation{{From: local(0), Type: "links", To: existing(target.ID)}},
			})
			require.ErrorIs(t, err, entitymanager.ErrEntityNotFound)
			op, ok := errors.AsType[*entitymanager.BatchOpError](err)
			require.True(t, ok)
			require.Equal(t, entitymanager.BatchEndTo, op.End)
			require.Zero(t, countNotes(t, st), "the batch entity is not written")
			require.Empty(t, successOps(sink.Records()))
		})
	}
}

// TestCreateBatch_RolledBackIDTakenByAnother pins that a batch the store
// rolled back reports nothing kept, even when another writer takes a
// freed generated id before the batch looks: that row is not audited as
// the batch's.
func TestCreateBatch_RolledBackIDTakenByAnother(t *testing.T) {
	for _, backend := range concBackends {
		if !rollsBack(backend) {
			continue
		}
		t.Run(backend.name, func(t *testing.T) {
			st, _, _ := openLinkedBackend(t, backend)
			hooked := &txHookStore{Store: st}
			hooked.after = func(ctx context.Context, err error) {
				if err == nil {
					return
				}
				e := entity.New("NOTE-1", "note")
				e.SetString("title", "someone else's")
				require.NoError(t, st.CreateEntity(ctx, e))
			}
			mgr, sink := managerOver(t, hooked)
			_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
				Entities: []entitymanager.BatchEntity{batchNote("a")},
				Relations: []entitymanager.BatchRelation{
					{From: local(0), Type: "links", To: local(0)},
					{From: local(0), Type: "links", To: local(0)},
				},
			})
			require.Error(t, err)
			_, kept := errors.AsType[*entitymanager.BatchKeptError](err)
			require.False(t, kept)
			require.Empty(t, successOps(sink.Records()))
		})
	}
}

// TestCreateBatch_IdentityRelationRefusesTail pins that a tail face on an
// existing source of an identity relation is refused, as a single create
// refuses it, rather than dropped.
func TestCreateBatch_IdentityRelationRefusesTail(t *testing.T) {
	_, mgr, _ := openLinkedBackend(t, concBackends[0])
	target := mustCreateNote(t, mgr, "target")
	_, err := entitymanager.CreateBatch(context.Background(), mgr, entitymanager.Batch{
		Entities: []entitymanager.BatchEntity{batchNote("a")},
		Relations: []entitymanager.BatchRelation{{
			From: entitymanager.BatchEnd{ID: target.ID, Face: "draft"}, Type: "links", To: local(0),
		}},
	})
	require.ErrorIs(t, err, entitymanager.ErrFaceNotDeclared)
}
