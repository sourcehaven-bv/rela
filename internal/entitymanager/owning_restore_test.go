package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// grantViaOwnerMetamodel has a person who owns tasks. The owning edge also
// confers a role on the task, so a task's delete grant comes through its
// owner.
const grantViaOwnerMetamodel = `version: "1.0"
entities:
  person:
    label: Person
    plural: persons
    id_type: manual
    properties:
      name: { type: string }
  note:
    label: Note
    plural: notes
    id_prefix: "NOTE-"
    id_type: sequential
    properties:
      title: { type: string }
relations:
  part:
    label: part
    from: [person]
    to: [note]
    owning: true
`

// Everyone may delete people and the edges from them. Only the owner may
// delete a note, through the role its owning edge confers.
const grantViaOwnerPolicy = `
roles:
  owner: { read: [note], delete: [note] }
  member: { read: [person], delete: [person] }
assignments:
  alice: member
  bob: member
role_relations:
  part: { confers: owner }
`

// TestRestore_OwnedGrantThroughOwner pins that a restore authorizes each owned
// entity with every family in the operation revealed. The owned note's delete
// grant comes only from the owning edge, which the owner's mark holds; a check
// that revealed only the note's own mark would refuse it.
//
// The review suggested inherit_roles_through for this. That cannot isolate
// the case: the inherited edge starts at the owned entity, and the cascade
// check authorizes an edge against a GLOBAL grant on its source type, which
// would grant the owned entity's delete by itself.
func TestRestore_OwnedGrantThroughOwner(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			st := b.open(t)
			meta, err := metamodel.Parse([]byte(grantViaOwnerMetamodel))
			require.NoError(t, err)
			seed := allowAllManager(t, st, meta, audit.Nop{})
			ctx := context.Background()
			for _, id := range []string{"alice", "bob"} {
				_, err = seed.CreateEntity(ctx, entity.New(id, "person"), entity.CreateOptions{ID: id})
				require.NoError(t, err)
			}
			child := createNote(t, seed, "child")
			mustLink(t, seed, "alice", "part", child)
			if !entitymanager.SupportsSoftDelete(seed) {
				t.Skip("backend has no soft delete")
			}

			policy, err := acl.LoadPolicyBytes([]byte(grantViaOwnerPolicy))
			require.NoError(t, err)
			decl, err := acl.NewDeclarative(policy, acl.NewStoreGraph(st), st)
			require.NoError(t, err)
			mgr, err := entitymanager.New(entitymanager.Deps{
				Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: decl,
				Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
				CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: allowAllCopyVisibility(t, st),
			})
			require.NoError(t, err)

			_, err = entitymanager.SoftDeleteEntity(userCtx("alice"), mgr, "alice")
			require.NoError(t, err)

			_, err = entitymanager.RestoreEntity(userCtx("bob"), mgr, "alice")
			var forbidden *acl.ForbiddenError
			require.ErrorAs(t, err, &forbidden, "bob holds no role on the note")

			_, err = entitymanager.RestoreEntity(userCtx("alice"), mgr, "alice")
			require.NoError(t, err)
			_, err = st.GetEntity(ctx, entity.Ref{ID: child})
			require.NoError(t, err, "the owned note is back")
			_, err = st.GetRelation(ctx, entity.RelationKey{From: "alice", Type: "part", To: child})
			require.NoError(t, err, "the owning edge is back")
		})
	}
}

// TestRestore_RefusesOwningEdgeTheLiveGraphForbids pins that a restore does
// not revive an owning edge that would give an entity a second owner. The
// owned entity was restored on its own and given a new owner meanwhile.
func TestRestore_RefusesOwningEdgeTheLiveGraphForbids(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			f := newOwnedDeleteFixture(t, b)
			if !entitymanager.SupportsSoftDelete(f.mgr) {
				t.Skip("backend has no soft delete")
			}
			ctx := aliceCtx()
			_, err := entitymanager.SoftDeleteEntity(ctx, f.mgr, f.owner)
			require.NoError(t, err)
			_, err = entitymanager.RestoreEntity(ctx, f.mgr, f.c1)
			require.NoError(t, err, "an owned entity can be restored on its own")
			newOwner := createNote(t, f.mgr, "new owner")
			mustLink(t, f.mgr, newOwner, "part", f.c1)

			_, err = entitymanager.RestoreEntity(ctx, f.mgr, f.owner)
			require.ErrorIs(t, err, entitymanager.ErrOwningRule)
			assert.NotContains(t, err.Error(), f.c1, "the refusal does not name the other end")
			assert.False(t, f.exists(t, f.owner), "a refused restore unmarks nothing")
			assert.False(t, f.exists(t, f.c2), "a refused restore unmarks nothing")
			assert.False(t, f.hasEdge(t, f.owner, "part", f.c1))
			assert.True(t, f.hasEdge(t, newOwner, "part", f.c1))
		})
	}
}

// TestCreateRelation_IdenticalOwningEdgeIsAConflict pins that re-creating an
// existing owning edge reports the duplicate, not an ownership clash.
func TestCreateRelation_IdenticalOwningEdgeIsAConflict(t *testing.T) {
	t.Parallel()
	mgr, st := newOwningManager(t)
	p, c := mkTask(t, mgr, "parent"), mkTask(t, mgr, "child")
	mustLink(t, mgr, p, "subtask", c)

	err := link(mgr, p, "subtask", c)
	require.ErrorIs(t, err, entitymanager.ErrRelationAlreadyExists)
	require.NotErrorIs(t, err, entitymanager.ErrOwningRule)

	// The check itself, as the create's Tx runs it after a concurrent writer
	// made the same edge: the identical edge is not a second owner.
	meta, err := metamodel.Parse([]byte(owningMetamodel))
	require.NoError(t, err)
	key := entity.RelationKey{From: p, Type: "subtask", To: c}
	require.NoError(t, entitymanager.CheckOwningEdge(context.Background(), meta, st, key))
	other := mkTask(t, mgr, "other")
	require.ErrorIs(t, entitymanager.CheckOwningEdge(context.Background(), meta, st,
		entity.RelationKey{From: other, Type: "subtask", To: c}), entitymanager.ErrOwningRule,
		"a different owner is still refused")
}

func createNote(t *testing.T, mgr *entitymanager.Manager, title string) string {
	t.Helper()
	e := entity.New("", "note")
	e.SetString("title", title)
	res, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	require.NoError(t, err)
	return res.Entity.ID
}

func allowAllManager(t *testing.T, st store.Store, meta *metamodel.Metamodel, sink audit.Audit) *entitymanager.Manager {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: sink,
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	require.NoError(t, err)
	return mgr
}

var errInjected = errors.New("injected failure")

// failingSoftDeletes fails MarkDeleted or Unmark on one id, inside or outside
// a transaction. It stands in for an I/O failure partway through an
// operation on a backend that cannot roll back.
type failingSoftDeletes struct {
	store.Store
	failMark, failUnmark *string
}

func (s failingSoftDeletes) SoftDelete() store.SoftDeleter {
	inner := s.Store.(store.SoftDeleteProvider).SoftDelete()
	return failingSoftDeleter{SoftDeleter: inner, failMark: s.failMark, failUnmark: s.failUnmark}
}

func (s failingSoftDeletes) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.Store.Tx(ctx, func(tx store.Store) error {
		return fn(failingSoftDeletes{Store: tx, failMark: s.failMark, failUnmark: s.failUnmark})
	})
}

type failingSoftDeleter struct {
	store.SoftDeleter
	failMark, failUnmark *string
}

func (d failingSoftDeleter) MarkDeleted(ctx context.Context, id, by string) (*store.DeleteResult, error) {
	if id == *d.failMark {
		return nil, errInjected
	}
	return d.SoftDeleter.MarkDeleted(ctx, id, by)
}

func (d failingSoftDeleter) Unmark(ctx context.Context, id string) (*store.DeleteResult, error) {
	if id == *d.failUnmark {
		return nil, errInjected
	}
	return d.SoftDeleter.Unmark(ctx, id)
}

// auditedEntityOps returns, per entity id, the entity ops the sink recorded.
func auditedEntityOps(sink *audit.Memory) map[string][]string {
	out := map[string][]string{}
	for _, r := range sink.Records() {
		if r.Subject != nil && (r.Op == audit.OpDeleteEntity || r.Op == audit.OpRestoreEntity) {
			out[r.Subject.ID] = append(out[r.Subject.ID], r.Op)
		}
	}
	return out
}

// TestSoftDelete_PartialFailureIsAudited pins that on a backend without
// rollback a soft delete or restore that fails partway still audits what it
// did change.
func TestSoftDelete_PartialFailureIsAudited(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends[:2] { // memstore and fsstore: no rollback
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			f := newOwnedDeleteFixture(t, b)
			meta, err := metamodel.Parse([]byte(ownedDeleteMetamodel))
			require.NoError(t, err)
			var failMark, failUnmark string
			sink := audit.NewMemory()
			mgr := allowAllManager(t, failingSoftDeletes{Store: f.st, failMark: &failMark, failUnmark: &failUnmark},
				meta, sink)
			ctx := aliceCtx()

			failMark = f.c2
			_, err = entitymanager.SoftDeleteEntity(ctx, mgr, f.owner)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), f.c2)
			ops := auditedEntityOps(sink)
			assert.Equal(t, []string{audit.OpDeleteEntity}, ops[f.owner], "the owner was marked")
			assert.Equal(t, []string{audit.OpDeleteEntity}, ops[f.c1], "c1 was marked")
			assert.Empty(t, ops[f.c2], "c2 was not marked")
			assert.True(t, f.exists(t, f.c2))

			// Mark c2 too, so the owner's restore finds both owned entities.
			failMark = ""
			_, err = entitymanager.SoftDeleteEntity(ctx, f.mgr, f.c2)
			require.NoError(t, err)
			sink2 := audit.NewMemory()
			mgr = allowAllManager(t, failingSoftDeletes{Store: f.st, failMark: &failMark, failUnmark: &failUnmark},
				meta, sink2)
			failUnmark = f.c2
			_, err = entitymanager.RestoreEntity(ctx, mgr, f.owner)
			require.Error(t, err)
			ops = auditedEntityOps(sink2)
			assert.Equal(t, []string{audit.OpRestoreEntity}, ops[f.c1], "c1 came back")
			assert.Empty(t, ops[f.c2], "c2 did not come back")
			assert.Empty(t, ops[f.owner], "the owner did not come back")
			assert.True(t, f.exists(t, f.c1))
			assert.False(t, f.exists(t, f.owner))
		})
	}
}

// copyOwningMetamodel copies a draft's owning edges onto a note, which would
// give the owned entity a second owner.
const copyOwningMetamodel = `version: "1.0"
entities:
  draft:
    label: Draft
    id_prefix: "DRAFT-"
    id_type: sequential
    properties:
      title: { type: string }
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: sequential
    properties:
      title: { type: string }
relations:
  part:
    label: part
    from: [draft, note]
    to: [note]
    owning: true
    scope: content
copies:
  publish:
    from: draft
    to: note
    fields:
      title: "{{new.title}}"
    relations:
      part: merge
`

// TestCopy_OwningEdgeFollowsTheRules pins that a copy's relation writes are
// held to the owning rules.
func TestCopy_OwningEdgeFollowsTheRules(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(copyOwningMetamodel))
	require.NoError(t, err)
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyGuard: allowGuard{allow: true},
	})
	require.NoError(t, err)
	ctx := aliceCtx()
	mk := func(typ, title string) string {
		e := entity.New("", typ)
		e.SetString("title", title)
		res, cErr := mgr.CreateEntity(ctx, e, entity.CreateOptions{})
		require.NoError(t, cErr)
		return res.Entity.ID
	}
	src, child, target := mk("draft", "source"), mk("note", "child"), mk("note", "target")
	mustLink(t, mgr, src, "part", child)

	_, err = mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "publish", SourceID: src, TargetID: target})
	require.ErrorIs(t, err, entitymanager.ErrOwningRule)
	_, err = st.GetRelation(context.Background(), entity.RelationKey{From: target, Type: "part", To: child})
	require.ErrorIs(t, err, store.ErrNotFound, "the copy gave child a second owner")
}

// failingFamilyDelete fails DeleteFamily on one id, inside a transaction.
type failingFamilyDelete struct {
	store.Store
	fail string
}

func (s failingFamilyDelete) DeleteFamily(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	if id == s.fail {
		return nil, errInjected
	}
	return s.Store.DeleteFamily(ctx, id, cascade)
}

func (s failingFamilyDelete) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.Store.Tx(ctx, func(tx store.Store) error {
		return fn(failingFamilyDelete{Store: tx, fail: s.fail})
	})
}

// TestDeleteEntity_PartialCascadeLabelsOwnedRelations pins that when an
// owner's delete fails after its owned entities went, on a backend without
// rollback, their relations carry the owner-delete label, as on success.
func TestDeleteEntity_PartialCascadeLabelsOwnedRelations(t *testing.T) {
	t.Parallel()
	for _, b := range concBackends[:2] { // memstore and fsstore: no rollback
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			f := newOwnedDeleteFixture(t, b)
			meta, err := metamodel.Parse([]byte(ownedDeleteMetamodel))
			require.NoError(t, err)
			sink := audit.NewMemory()
			mgr := allowAllManager(t, failingFamilyDelete{Store: f.st, fail: f.owner}, meta, sink)

			_, err = mgr.DeleteEntity(aliceCtx(), f.owner, true)
			require.ErrorIs(t, err, errInjected)
			require.False(t, f.exists(t, f.c1), "the owned entity went before the failure")

			want := "cascade:owner-delete:" + f.owner
			var labeled int
			for _, r := range sink.Records() {
				if r.Op != audit.OpDeleteRelation {
					continue
				}
				labeled++
				assert.Equal(t, want, r.TriggeredBy, "relation record %s", r.Summary)
			}
			assert.Positive(t, labeled, "the owned entities' relations were recorded")
		})
	}
}
