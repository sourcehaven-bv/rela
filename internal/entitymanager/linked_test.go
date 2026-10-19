package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// rollsBack reports whether backend b's Tx undoes the writes of a failed
// callback.
func rollsBack(b concBackend) bool {
	return b.name == "sqlite" || strings.HasPrefix(b.name, "postgres")
}

// openLinkedBackend is openConcBackend with an audit sink.
func openLinkedBackend(t *testing.T, b concBackend) (store.Store, *entitymanager.Manager, *audit.Memory) {
	t.Helper()
	st := b.open(t)
	meta, err := metamodel.Parse([]byte(concurrencyMetamodelYAML))
	require.NoError(t, err)
	sink := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: sink, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	require.NoError(t, err)
	return st, mgr, sink
}

// linkedAuditOps lists the ops of the audit records about id, in order.
func linkedAuditOps(sink *audit.Memory, id string) []string {
	var ops []string
	for _, r := range sink.Records() {
		if r.Subject != nil && (r.Subject.ID == id || r.Subject.FromID == id) {
			ops = append(ops, r.Op)
		}
	}
	return ops
}

// TestCreateLinked_CommitsTogether pins that CreateLinked writes the
// entity and its edge, and runs InTx on a view that sees both.
func TestCreateLinked_CommitsTogether(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, sink := openLinkedBackend(t, backend)
			target := mustCreateNote(t, mgr, "target")
			e := entity.New("", "note")
			e.SetString("title", "new")
			var seen string
			res, err := entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
				Entity: e, RelateType: "links", RelateTo: target.ID,
				InTx: func(ctx context.Context, view store.Store, created *entity.Entity) error {
					got, err := view.GetEntity(ctx, created.Ref())
					if err != nil {
						return err
					}
					_, err = view.GetRelation(ctx, entity.RelationKey{From: created.ID, Type: "links", To: target.ID})
					seen = got.ID
					return err
				},
			})
			require.NoError(t, err)
			require.NoError(t, res.FollowUpErr)
			require.NoError(t, res.RelateErr)
			id := res.Create.Entity.ID
			require.Equal(t, id, seen)
			require.NotNil(t, res.Relation)
			_, err = st.GetRelation(context.Background(), entity.RelationKey{From: id, Type: "links", To: target.ID})
			require.NoError(t, err)
			require.Equal(t, []string{audit.OpCreateEntity, audit.OpCreateRelation}, linkedAuditOps(sink, id),
				"the entity record comes before the edge record")
		})
	}
}

// TestCreateLinked_InTxErrorRollsBack pins the atomicity: when InTx fails,
// a store with a real transaction keeps neither the entity nor the edge.
// The filesystem and memory stores keep what they wrote, and audit it.
func TestCreateLinked_InTxErrorRollsBack(t *testing.T) {
	boom := errors.New("mark failed")
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			st, mgr, sink := openLinkedBackend(t, backend)
			target := mustCreateNote(t, mgr, "target")
			e := entity.New("", "note")
			e.SetString("title", "new")
			var id string
			_, err := entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
				Entity: e, RelateType: "links", RelateTo: target.ID,
				InTx: func(_ context.Context, _ store.Store, created *entity.Entity) error {
					id = created.ID
					return boom
				},
			})
			require.ErrorIs(t, err, boom)
			_, getErr := st.GetEntity(context.Background(), entity.Ref{ID: id})
			_, relErr := st.GetRelation(context.Background(), entity.RelationKey{From: id, Type: "links", To: target.ID})
			if rollsBack(backend) {
				require.ErrorIs(t, getErr, store.ErrNotFound)
				require.ErrorIs(t, relErr, store.ErrNotFound)
				require.Empty(t, linkedAuditOps(sink, id), "nothing is audited for a rolled-back create")
			} else {
				require.NoError(t, getErr)
				require.NoError(t, relErr)
				require.Equal(t, []string{audit.OpCreateEntity, audit.OpCreateRelation}, linkedAuditOps(sink, id),
					"a kept entity and edge are audited")
			}
		})
	}
}

// TestCreateLinked_RelationCheckedBeforeWrite pins that an edge the
// metamodel refuses fails before the entity is written, on every store.
func TestCreateLinked_RelationCheckedBeforeWrite(t *testing.T) {
	mgr, cs := newManager(t, nil)
	req := createReq(t, mgr, "r")
	before := cs.creates.Load()
	e := entity.New("", "decision")
	e.SetString("title", "d")
	_, err := entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
		Entity: e, RelateType: "covers", RelateTo: req.ID,
	})
	var invalid *entitymanager.InvalidRelationError
	require.ErrorAs(t, err, &invalid)
	require.True(t, entitymanager.WroteNothing(err))
	_, err = entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{
		Entity: e, RelateType: "addresses", RelateTo: "REQ-404",
	})
	require.ErrorIs(t, err, entitymanager.ErrEntityNotFound)
	require.Equal(t, before, cs.creates.Load(), "no entity is written")
}

// failingRelStore refuses every relation create, also on its Tx views.
type failingRelStore struct {
	store.Store
	err error
}

func (s *failingRelStore) CreateRelation(
	context.Context, entity.RelationKey, *store.RelationData,
) (*entity.Relation, error) {
	return nil, s.err
}

func (s *failingRelStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.Store.Tx(ctx, func(view store.Store) error { return fn(&failingRelStore{Store: view, err: s.err}) })
}

// TestCreateLinked_RelateBestEffort pins that RelateBestEffort keeps the
// entity and reports the edge failure, and that without it the failure is
// the error.
func TestCreateLinked_RelateBestEffort(t *testing.T) {
	boom := errors.New("edge write failed")
	for _, bestEffort := range []bool{true, false} {
		mgr, cs := newManager(t, nil)
		req := createReq(t, mgr, "r")
		failing := entitymanager.WithStore(mgr, &failingRelStore{Store: cs, err: boom})
		e := entity.New("", "decision")
		e.SetString("title", "d")
		res, err := entitymanager.CreateLinked(context.Background(), failing, entitymanager.LinkedCreate{
			Entity: e, RelateType: "addresses", RelateTo: req.ID, RelateBestEffort: bestEffort,
		})
		if bestEffort {
			require.NoError(t, err)
			require.ErrorIs(t, res.RelateErr, boom)
			require.Nil(t, res.Relation)
			require.NotEmpty(t, res.Create.Entity.ID)
		} else {
			require.ErrorIs(t, err, boom)
			require.False(t, entitymanager.WroteNothing(err), "the edge failed inside the transaction")
		}
	}
}

// TestCreateLinked_RefusedWriteWroteNothing pins that a refusal of the
// entity write inside the transaction, here a unique collision, is
// reported as having written nothing, on every store.
func TestCreateLinked_RefusedWriteWroteNothing(t *testing.T) {
	for _, backend := range concBackends {
		t.Run(backend.name, func(t *testing.T) {
			_, mgr, sink := openLinkedBackend(t, backend)
			first := entity.New("", "person")
			first.SetString("email", "a@example.com")
			_, err := mgr.CreateEntity(context.Background(), first, entity.CreateOptions{})
			require.NoError(t, err)
			before := len(sink.Records())
			dup := entity.New("", "person")
			dup.SetString("email", "a@example.com")
			_, err = entitymanager.CreateLinked(context.Background(), mgr, entitymanager.LinkedCreate{Entity: dup})
			require.Error(t, err)
			require.True(t, entitymanager.WroteNothing(err), "%v", err)
			require.Len(t, sink.Records(), before, "a refused create is not audited")
		})
	}
}

// TestCheckCreate pins that the probe answers the create's ACL question
// and leaves no audit record, allowed or not.
func TestCheckCreate(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	denied, _ := newManagerWithACL(t, acl.ReadOnlyACL{}, sink)
	var forbidden *acl.ForbiddenError
	require.ErrorAs(t, entitymanager.CheckCreate(context.Background(), denied, "decision", ""), &forbidden)
	require.Empty(t, sink.Records(), "a probe is not an attempted write")
	allowed, _ := newManagerWithACL(t, acl.NopACL{}, sink)
	require.NoError(t, entitymanager.CheckCreate(context.Background(), allowed, "decision", ""))
}
