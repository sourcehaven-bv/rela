package entitymanager_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

type recordedVersions struct {
	mu   sync.Mutex
	recs []entitymanager.VersionRecord
}

func (r *recordedVersions) RecordVersion(_ context.Context, v entitymanager.VersionRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, v)
	return nil
}

type softDeleteFixture struct {
	mgr      *entitymanager.Manager
	st       *memstore.MemStore
	sink     *audit.Memory
	versions *recordedVersions
	reqID    string
	decID    string
}

// newSoftDeleteFixture seeds a decision that addresses a requirement.
func newSoftDeleteFixture(t *testing.T) softDeleteFixture {
	t.Helper()
	st := memstore.New()
	sink := audit.NewMemory()
	versions := &recordedVersions{}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: parseMeta(t), Templater: nopTemplater{}, Audit: sink,
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{}, VersionRecorder: versions,
	})
	require.NoError(t, err)
	ctx := context.Background()
	req := entity.New("", "requirement")
	req.SetString("title", "R")
	reqRes, err := mgr.CreateEntity(ctx, req, entity.CreateOptions{})
	require.NoError(t, err)
	dec := entity.New("", "decision")
	dec.SetString("title", "D")
	decRes, err := mgr.CreateEntity(ctx, dec, entity.CreateOptions{})
	require.NoError(t, err)
	_, err = mgr.CreateRelation(ctx, entity.RelationKey{From: decRes.Entity.ID, Type: "addresses", To: reqRes.Entity.ID}, entity.RelationOptions{})
	require.NoError(t, err)
	return softDeleteFixture{mgr: mgr, st: st, sink: sink, versions: versions,
		reqID: reqRes.Entity.ID, decID: decRes.Entity.ID}
}

func aliceCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
}

func auditOps(sink *audit.Memory, from int) []string {
	var ops []string
	for _, r := range sink.Records()[from:] {
		ops = append(ops, r.Op+" "+r.Summary)
	}
	return ops
}

func TestSoftDelete_RestoreRoundTrip(t *testing.T) {
	t.Parallel()
	f := newSoftDeleteFixture(t)
	ctx := aliceCtx()

	before := len(f.sink.Records())
	res, err := entitymanager.SoftDeleteEntity(ctx, f.mgr, f.reqID)
	require.NoError(t, err)
	assert.Len(t, res.DeletedRelations, 1)
	assert.Equal(t, []string{"delete-relation deleted (undoable)", "delete-entity deleted (undoable)"},
		auditOps(f.sink, before))
	assert.Empty(t, f.versions.recs, "a mark records no version; an undo must leave none behind")

	_, err = f.st.GetEntity(ctx, entity.Ref{ID: f.reqID})
	require.ErrorIs(t, err, store.ErrNotFound)
	found, ok, err := entitymanager.FindSoftDeleted(ctx, f.mgr, f.reqID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, entitymanager.SoftDeleted{ID: f.reqID, Type: "requirement", DeletedBy: "alice"}, found)

	before = len(f.sink.Records())
	_, err = entitymanager.RestoreEntity(ctx, f.mgr, f.reqID)
	require.NoError(t, err)
	assert.Equal(t, []string{"create-relation restored", "restore-entity restored"}, auditOps(f.sink, before))
	got, err := f.st.GetEntity(ctx, entity.Ref{ID: f.reqID})
	require.NoError(t, err)
	assert.Equal(t, "R", got.GetString("title"))
	_, err = f.st.GetRelation(ctx, entity.RelationKey{From: f.decID, Type: "addresses", To: f.reqID})
	require.NoError(t, err)

	_, err = entitymanager.RestoreEntity(ctx, f.mgr, f.reqID)
	require.ErrorIs(t, err, entitymanager.ErrEntityNotFound, "a second undo finds nothing")
}

func TestSoftDelete_PurgeRecordsTheDeletersAction(t *testing.T) {
	t.Parallel()
	f := newSoftDeleteFixture(t)
	_, err := entitymanager.SoftDeleteEntity(aliceCtx(), f.mgr, f.reqID)
	require.NoError(t, err)

	// Not yet old enough.
	n, err := entitymanager.PurgeSoftDeleted(context.Background(), f.mgr, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Zero(t, n)

	before := len(f.sink.Records())
	n, err = entitymanager.PurgeSoftDeleted(context.Background(), f.mgr, time.Now().Add(time.Second))
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	recs := f.sink.Records()[before:]
	require.Len(t, recs, 1)
	assert.Equal(t, audit.OpPurgeDeletedEntity, recs[0].Op)
	assert.Equal(t, "alice", recs[0].Principal.User)
	assert.Equal(t, principal.ToolSoftDeleteGC, recs[0].Principal.Tool)

	require.Len(t, f.versions.recs, 1)
	v := f.versions.recs[0]
	assert.Equal(t, store.VersionOpDelete, v.Op)
	assert.Equal(t, f.reqID, v.EntityID)
	assert.Equal(t, "alice", v.PrincipalUser)

	_, err = entitymanager.RestoreEntity(aliceCtx(), f.mgr, f.reqID)
	require.ErrorIs(t, err, entitymanager.ErrEntityNotFound)
	_, ok, err := entitymanager.FindSoftDeleted(context.Background(), f.mgr, f.reqID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSoftDelete_UnsupportedStore(t *testing.T) {
	t.Parallel()
	mgr, _ := newManagerWithACL(t, acl.NopACL{}, audit.Nop{})
	assert.False(t, entitymanager.SupportsSoftDelete(mgr))
	_, err := entitymanager.SoftDeleteEntity(context.Background(), mgr, "REQ-001")
	require.ErrorIs(t, err, entitymanager.ErrSoftDeleteUnsupported)
	_, err = entitymanager.RestoreEntity(context.Background(), mgr, "REQ-001")
	require.ErrorIs(t, err, entitymanager.ErrSoftDeleteUnsupported)
}

// A soft-deleted entity keeps its unique values, so the undo cannot be
// blocked by someone taking the value meanwhile.
func TestSoftDelete_HoldsUniqueValues(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_prefix: "P-"
    id_type: sequential
    properties:
      email: {type: string, unique: true}
`))
	require.NoError(t, err)
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: memstore.New(), Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	require.NoError(t, err)
	ctx := context.Background()
	p := entity.New("", "person")
	p.SetString("email", "a@example.com")
	res, err := mgr.CreateEntity(ctx, p, entity.CreateOptions{})
	require.NoError(t, err)

	_, err = entitymanager.SoftDeleteEntity(ctx, mgr, res.Entity.ID)
	require.NoError(t, err)

	dup := entity.New("", "person")
	dup.SetString("email", "a@example.com")
	_, err = mgr.CreateEntity(ctx, dup, entity.CreateOptions{})
	var vErr *entitymanager.ValidationError
	require.ErrorAs(t, err, &vErr, "the value is still held by the soft-deleted entity")

	_, err = entitymanager.RestoreEntity(ctx, mgr, res.Entity.ID)
	require.NoError(t, err)
}

// newRestoreACLFixture seeds a ticket assigned to alice, behind a manager
// that enforces this policy:
//
//   - alice holds the ticket delete grant only locally, through the
//     assigned-to edge, and a global role for the edge's own delete check;
//   - bob holds no role;
//   - carol may delete tickets globally but has no grant on person-sourced
//     relations.
func newRestoreACLFixture(t *testing.T) (*entitymanager.Manager, *memstore.MemStore, string) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_type: manual
    properties:
      name: { type: string }
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title: { type: string }
relations:
  assigned-to:
    label: Assigned to
    from: [person]
    to: [ticket]
`))
	require.NoError(t, err)
	policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  assignee: { read: [ticket, assigned-to], delete: [ticket, assigned-to] }
  linker: { read: [person], delete: [person] }
  ticket-admin: { read: [ticket], delete: [ticket] }
assignments:
  alice: linker
  carol: ticket-admin
role_relations:
  assigned-to: { confers: assignee }
`))
	require.NoError(t, err)

	st := memstore.New()
	seed, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	require.NoError(t, err)
	ctx := context.Background()
	for _, id := range []string{"alice", "bob", "carol"} {
		_, err = seed.CreateEntity(ctx, entity.New(id, "person"), entity.CreateOptions{ID: id})
		require.NoError(t, err)
	}
	tkt, err := seed.CreateEntity(ctx, entity.New("", "ticket"), entity.CreateOptions{})
	require.NoError(t, err)
	_, err = seed.CreateRelation(ctx, entity.RelationKey{From: "alice", Type: "assigned-to", To: tkt.Entity.ID}, entity.RelationOptions{})
	require.NoError(t, err)

	declarative, err := acl.NewDeclarative(policy, acl.NewStoreGraph(st), st)
	require.NoError(t, err)
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: declarative,
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyReadGate: entitymanager.AllowAllCopyReadGate{}, CopyVisibility: allowAllCopyVisibility(t, st),
	})
	require.NoError(t, err)
	return mgr, st, tkt.Entity.ID
}

func userCtx(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
}

// Restore is authorized as a delete, against the local roles the entity had
// when it was deleted. Those come from relations that are hidden while it is
// marked, so this passes only because the check runs under the reveal.
func TestSoftDelete_RestoreUsesLocalRolesThroughReveal(t *testing.T) {
	t.Parallel()
	mgr, st, tktID := newRestoreACLFixture(t)

	_, err := entitymanager.SoftDeleteEntity(userCtx("alice"), mgr, tktID)
	require.NoError(t, err)

	_, err = entitymanager.RestoreEntity(userCtx("bob"), mgr, tktID)
	var forbidden *acl.ForbiddenError
	require.ErrorAs(t, err, &forbidden, "bob holds no role on the ticket")

	_, err = entitymanager.RestoreEntity(userCtx("alice"), mgr, tktID)
	require.NoError(t, err, "alice's role comes from the hidden assigned-to edge")
	_, err = st.GetRelation(context.Background(), entity.RelationKey{From: "alice", Type: "assigned-to", To: tktID})
	require.NoError(t, err)
}

// A restore needs the relation grants the delete needed. carol may delete the
// ticket itself, but not the person-sourced edge that would come back with
// it, so she is refused and the ticket stays deleted.
func TestSoftDelete_RestoreChecksRelationGrants(t *testing.T) {
	t.Parallel()
	mgr, st, tktID := newRestoreACLFixture(t)

	_, err := entitymanager.SoftDeleteEntity(userCtx("carol"), mgr, tktID)
	var forbidden *acl.ForbiddenError
	require.ErrorAs(t, err, &forbidden, "the delete itself needs the edge grant")

	_, err = entitymanager.SoftDeleteEntity(userCtx("alice"), mgr, tktID)
	require.NoError(t, err)

	_, err = entitymanager.RestoreEntity(userCtx("carol"), mgr, tktID)
	require.ErrorAs(t, err, &forbidden, "carol has no delete grant on assigned-to from person")

	_, found, err := entitymanager.FindSoftDeleted(context.Background(), mgr, tktID)
	require.NoError(t, err)
	assert.True(t, found, "a refused restore unmarks nothing")
	_, err = st.GetEntity(context.Background(), entity.Ref{ID: tktID})
	require.ErrorIs(t, err, store.ErrNotFound)
	_, err = st.GetRelation(context.Background(), entity.RelationKey{From: "alice", Type: "assigned-to", To: tktID})
	require.ErrorIs(t, err, store.ErrNotFound)
}
