package entitymanager_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// cardinalityManager is a manager over a task/status schema (TKT-65LVAK):
// has_status allows one status per task, and owns allows one task per owner
// on the incoming side.
func cardinalityManager(t *testing.T) (*entitymanager.Manager, store.Store) {
	t.Helper()
	return cardinalityManagerWith(t, audit.Nop{}, nil)
}

// cardinalityManagerWith is cardinalityManager with the audit sink and
// relation version recorder given.
func cardinalityManagerWith(
	t *testing.T, aud audit.Audit, rec entitymanager.RelationVersionRecorder,
) (*entitymanager.Manager, store.Store) {
	t.Helper()
	return cardinalityManagerACL(t, aud, rec, acl.NopACL{})
}

// cardinalityManagerACL is cardinalityManagerWith under the given ACL.
func cardinalityManagerACL(
	t *testing.T, aud audit.Audit, rec entitymanager.RelationVersionRecorder, a acl.ACL,
) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(`
entities:
  task:
    label: Task
    id_prefix: T
    properties:
      title: {type: string}
  status:
    label: Status
    id_prefix: S
    properties:
      title: {type: string}
  doc:
    label: Doc
    id_prefix: D
    faces:
      draft: {}
      published: {}
    properties:
      title: {type: string}
relations:
  has_status:
    from: [task]
    to: [status]
    max_outgoing: 1
  exclusive_status:
    from: [task]
    to: [status]
    max_outgoing: 1
    max_incoming: 1
  tagged:
    from: [task]
    to: [status]
  two_statuses:
    from: [task]
    to: [status]
    max_outgoing: 2
  cites:
    scope: content
    from: [doc]
    to: [status]
    max_outgoing: 1
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL: a, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{}, RelationVersionRecorder: rec,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "T-1", Type: "task"}, {ID: "T-2", Type: "task"},
		{ID: "S-1", Type: "status"}, {ID: "S-2", Type: "status"},
		{ID: "S-3", Type: "status"}, {ID: "S-4", Type: "status"},
		{ID: "D-1", Type: "doc", Face: "draft"}, {ID: "D-1", Type: "doc", Face: "published"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	return mgr, st
}

func edgesFrom(t *testing.T, st store.Store, from, relType string) []string {
	t.Helper()
	var out []string
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{From: from, Type: relType}) {
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		out = append(out, r.To)
	}
	return out
}

func key(from, relType, to string) entity.RelationKey {
	return entity.RelationKey{From: from, Type: relType, To: to}
}

func TestCreateRelation_RejectsSecondEdgeOverMaxOutgoing(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-2"), entity.RelationOptions{})
	var ce *entitymanager.CardinalityError
	if !errors.As(err, &ce) || !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("second create err = %v, want a CardinalityError", err)
	}
	if ce.Relation != "has_status" || ce.Constraint != "max_outgoing" || ce.Limit != 1 || ce.Entity != "T-1" {
		t.Errorf("error = %+v", ce)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
}

func TestCreateRelation_RejectsOverMaxIncoming(t *testing.T) {
	mgr, _ := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "exclusive_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := mgr.CreateRelation(ctx, key("T-2", "exclusive_status", "S-1"), entity.RelationOptions{})
	var ce *entitymanager.CardinalityError
	if !errors.As(err, &ce) || ce.Constraint != "max_incoming" || ce.Entity != "S-1" {
		t.Fatalf("err = %v, want max_incoming on S-1", err)
	}
}

func TestCreateRelation_UnboundedRelationAllowsMany(t *testing.T) {
	mgr, _ := cardinalityManager(t)
	ctx := context.Background()
	for _, to := range []string{"S-1", "S-2"} {
		if _, err := mgr.CreateRelation(ctx, key("T-1", "tagged", to), entity.RelationOptions{}); err != nil {
			t.Fatalf("create tagged %s: %v", to, err)
		}
	}
}

// replace runs ReplaceRelations with one create and the given removes.
func replace(
	ctx context.Context, mgr *entitymanager.Manager, create entity.RelationKey, removes ...entity.RelationKey,
) error {
	_, err := mgr.ReplaceRelations(ctx, []entitymanager.RelationCreate{{Key: create}}, removes)
	return err
}

func TestReplaceRelations_RepointsToExactlyOneEdge(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	if err := replace(ctx, mgr, key("T-1", "has_status", "S-2"), key("T-1", "has_status", "S-1")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-2" {
		t.Errorf("edges = %v, want [S-2]", got)
	}
}

func TestReplaceRelations_CreatesWhenRemovedEdgeIsGone(t *testing.T) {
	mgr, st := cardinalityManager(t)
	if err := replace(context.Background(), mgr, key("T-1", "has_status", "S-1"),
		key("T-1", "has_status", "S-2")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
}

// A create whose edge exists already fails, so the caller's 409 and retry
// path runs instead of the request's properties being dropped.
func TestReplaceRelations_ExistingCreateIsAlreadyExists(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	err := replace(ctx, mgr, key("T-1", "has_status", "S-1"))
	if !errors.Is(err, entitymanager.ErrRelationAlreadyExists) {
		t.Fatalf("err = %v, want ErrRelationAlreadyExists", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
}

// Data over the bound (written around the manager, as a loaded file can be)
// is repaired by a replace rather than refused.
func TestReplaceRelations_RepairsDataOverTheBound(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	for _, to := range []string{"S-1", "S-2"} {
		if _, err := st.CreateRelation(ctx, key("T-1", "has_status", to), nil); err != nil {
			t.Fatalf("seed %s: %v", to, err)
		}
	}
	if err := replace(ctx, mgr, key("T-1", "has_status", "S-3"),
		key("T-1", "has_status", "S-1"), key("T-1", "has_status", "S-2")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-3" {
		t.Errorf("edges = %v, want [S-3]", got)
	}
}

// An edge the caller does not name is not removed: the caller, not the
// manager, knows which edges the user may see.
func TestReplaceRelations_LeavesUnnamedEdges(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := st.CreateRelation(ctx, key("T-1", "tagged", "S-1"), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := replace(ctx, mgr, key("T-1", "tagged", "S-2")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "tagged"); len(got) != 2 {
		t.Errorf("edges = %v, want both S-1 and S-2", got)
	}
}

// Two new targets replace two old ones on a max_outgoing: 2 side: the
// removed edges do not count, and the first create counts against the
// second.
func TestReplaceRelations_TwoCreatesWithinMaxTwo(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	for _, to := range []string{"S-1", "S-2"} {
		if _, err := mgr.CreateRelation(ctx, key("T-1", "two_statuses", to), entity.RelationOptions{}); err != nil {
			t.Fatalf("seed %s: %v", to, err)
		}
	}
	_, err := mgr.ReplaceRelations(ctx,
		[]entitymanager.RelationCreate{{Key: key("T-1", "two_statuses", "S-3")}, {Key: key("T-1", "two_statuses", "S-4")}},
		[]entity.RelationKey{key("T-1", "two_statuses", "S-1"), key("T-1", "two_statuses", "S-2")})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "two_statuses"); len(got) != 2 || got[0] != "S-3" || got[1] != "S-4" {
		t.Errorf("edges = %v, want [S-3 S-4]", got)
	}
	// Removing only one of the two leaves no room for two new edges.
	_, err = mgr.ReplaceRelations(ctx,
		[]entitymanager.RelationCreate{{Key: key("T-1", "two_statuses", "S-1")}, {Key: key("T-1", "two_statuses", "S-2")}},
		[]entity.RelationKey{key("T-1", "two_statuses", "S-3")})
	if !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("err = %v, want ErrCardinalityExceeded", err)
	}
	if got := edgesFrom(t, st, "T-1", "two_statuses"); len(got) != 2 || got[0] != "S-3" || got[1] != "S-4" {
		t.Errorf("edges = %v, want [S-3 S-4] unchanged", got)
	}
}

// A max_incoming bound is re-pointed from the target side: the removed edge
// has another source than the created one.
func TestReplaceRelations_RepointsIncomingSide(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "exclusive_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := replace(ctx, mgr, key("T-2", "exclusive_status", "S-1"), key("T-1", "exclusive_status", "S-1")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := edgesFrom(t, st, "T-1", "exclusive_status"); len(got) != 0 {
		t.Errorf("T-1 edges = %v, want none", got)
	}
	if got := edgesFrom(t, st, "T-2", "exclusive_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("T-2 edges = %v, want [S-1]", got)
	}
}

func TestReplaceRelations_RefusesKeyNamedTwice(t *testing.T) {
	mgr, _ := cardinalityManager(t)
	err := replace(context.Background(), mgr, key("T-1", "has_status", "S-1"), key("T-1", "has_status", "S-1"))
	if err == nil {
		t.Fatal("replace creating and removing one edge succeeded")
	}
}

// A refused replace changes no edge and records no version and no audit.
func TestReplaceRelations_RefusedCreateKeepsOriginalEdge(t *testing.T) {
	aud := &recordingAudit{}
	rec := &fakeRelationRecorder{}
	mgr, st := cardinalityManagerWith(t, aud, rec)
	ctx := context.Background()
	for _, k := range []entity.RelationKey{
		key("T-1", "exclusive_status", "S-1"), key("T-2", "exclusive_status", "S-2"),
	} {
		if _, err := mgr.CreateRelation(ctx, k, entity.RelationOptions{}); err != nil {
			t.Fatalf("seed %v: %v", k, err)
		}
	}
	audits := aud.count()
	// S-2 already holds its one incoming edge, so the create half is refused.
	err := replace(ctx, mgr, key("T-1", "exclusive_status", "S-2"), key("T-1", "exclusive_status", "S-1"))
	if !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("err = %v, want ErrCardinalityExceeded", err)
	}
	if got := edgesFrom(t, st, "T-1", "exclusive_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want the original [S-1]", got)
	}
	if len(rec.records) != 0 {
		t.Errorf("versions = %+v, want none", rec.records)
	}
	if got := aud.ops()[audits:]; len(got) != 0 {
		t.Errorf("audit = %v, want none", got)
	}
}

// A successful replace records the removed edge's delete version and audits
// both halves.
func TestReplaceRelations_RecordsVersionAndAudit(t *testing.T) {
	aud := &recordingAudit{}
	rec := &fakeRelationRecorder{}
	mgr, _ := cardinalityManagerWith(t, aud, rec)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	audits := aud.count()
	if err := replace(ctx, mgr, key("T-1", "has_status", "S-2"), key("T-1", "has_status", "S-1")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(rec.records) != 1 || rec.records[0].Op != store.VersionOpDelete || rec.records[0].To != "S-1" {
		t.Errorf("versions = %+v, want one delete of T-1 -> S-1", rec.records)
	}
	got := aud.ops()[audits:]
	if len(got) != 2 || got[0] != audit.OpDeleteRelation || got[1] != audit.OpCreateRelation {
		t.Errorf("audit = %v, want delete then create", got)
	}
}

func TestReplaceRelations_MissingTargetChangesNothing(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	err := replace(ctx, mgr, key("T-1", "has_status", "S-9"), key("T-1", "has_status", "S-1"))
	if !errors.Is(err, entitymanager.ErrEntityNotFound) {
		t.Fatalf("err = %v, want ErrEntityNotFound", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
}

// An automation's create_relation is refused over the bound too; an edge it
// finds present is still the idempotent no-op.
func TestCascadeWriteRelation_RefusedOverMaxOutgoing(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	if err := entitymanager.CascadeHostWriteRelation(ctx, mgr, entity.NewRelation("T-1", "has_status", "S-1")); err != nil {
		t.Fatalf("re-writing the present edge: %v", err)
	}
	err := entitymanager.CascadeHostWriteRelation(ctx, mgr, entity.NewRelation("T-1", "has_status", "S-2"))
	if !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("err = %v, want ErrCardinalityExceeded", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1]", got)
	}
	if err := entitymanager.CascadeHostWriteRelation(ctx, mgr, entity.NewRelation("T-1", "tagged", "S-2")); err != nil {
		t.Fatalf("unbounded write: %v", err)
	}
}

// lineageRecorder is a fakeRelationRecorder that also answers lineage ids,
// as the postgres and sqlite recorders do.
type lineageRecorder struct {
	fakeRelationRecorder
	ids map[entity.RelationKey]int64
}

func (r *lineageRecorder) RelationRecordID(_ context.Context, k entity.RelationKey) (int64, error) {
	return r.ids[k], nil
}

// The lineage id is read before the delete and reaches the recorder, so the
// delete version lands on the edge's own lineage.
func TestReplaceRelations_PassesLineageIDToRecorder(t *testing.T) {
	rec := &lineageRecorder{ids: map[entity.RelationKey]int64{key("T-1", "has_status", "S-1"): 42}}
	mgr, _ := cardinalityManagerWith(t, audit.Nop{}, rec)
	ctx := context.Background()
	if _, err := mgr.CreateRelation(ctx, key("T-1", "has_status", "S-1"), entity.RelationOptions{}); err != nil {
		t.Fatalf("seed edge: %v", err)
	}
	if err := replace(ctx, mgr, key("T-1", "has_status", "S-2"), key("T-1", "has_status", "S-1")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(rec.records) != 1 || rec.records[0].RecordID != 42 {
		t.Errorf("versions = %+v, want one with RecordID 42", rec.records)
	}
}

// denyDeleteACL allows every write but a delete.
type denyDeleteACL struct{ acl.NopACL }

func (denyDeleteACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if req.Op == acl.OpDelete {
		return acl.Decision{Allow: false, RuleKind: "test", Reason: "no deletes"}
	}
	return acl.Decision{Allow: true}
}

// A denied remove refuses the whole replace before anything is written.
func TestReplaceRelations_DeniedRemoveWritesNothing(t *testing.T) {
	mgr, st := cardinalityManagerACL(t, audit.Nop{}, nil, denyDeleteACL{})
	ctx := context.Background()
	if _, err := st.CreateRelation(ctx, key("T-1", "has_status", "S-1"), nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := replace(ctx, mgr, key("T-1", "has_status", "S-2"), key("T-1", "has_status", "S-1"))
	if _, denied := errors.AsType[*acl.ForbiddenError](err); !denied {
		t.Fatalf("err = %v, want a ForbiddenError", err)
	}
	if got := edgesFrom(t, st, "T-1", "has_status"); len(got) != 1 || got[0] != "S-1" {
		t.Errorf("edges = %v, want [S-1] unchanged", got)
	}
}

// A content-scoped bound counts per face: the published face has room while
// the draft face is full, and a replace on the draft face frees its slot.
func TestReplaceRelations_ContentScopeCountsPerFace(t *testing.T) {
	mgr, st := cardinalityManager(t)
	ctx := context.Background()
	draft := entity.RelationKey{From: "D-1", FromFace: "draft", Type: "cites", To: "S-1"}
	if _, err := mgr.CreateRelation(ctx, draft, entity.RelationOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	published := entity.RelationKey{From: "D-1", FromFace: "published", Type: "cites", To: "S-2"}
	if err := replace(ctx, mgr, published); err != nil {
		t.Fatalf("create on the other face: %v", err)
	}
	draft2 := entity.RelationKey{From: "D-1", FromFace: "draft", Type: "cites", To: "S-3"}
	if err := replace(ctx, mgr, draft2); !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("second draft edge: err = %v, want ErrCardinalityExceeded", err)
	}
	if err := replace(ctx, mgr, draft2, draft); err != nil {
		t.Fatalf("re-point the draft face: %v", err)
	}
	var got []string
	for r, err := range st.ListRelations(ctx, store.RelationQuery{From: "D-1", Type: "cites"}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		got = append(got, string(r.FromFace)+">"+r.To)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"draft>S-3", "published>S-2"}) {
		t.Errorf("edges = %v, want [draft>S-3 published>S-2]", got)
	}
}
