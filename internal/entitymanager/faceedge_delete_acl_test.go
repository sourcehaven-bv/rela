package entitymanager_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// faceEdgeMetaYAML is the e2e faced fixture's shape: a policy stored only at
// named faces, with a content-scoped edge per face to a faceless control.
const faceEdgeMetaYAML = `version: "1.0"
entities:
  policy:
    label: Policy
    plural: policies
    id_prefix: "POL-"
    faces:
      draft: {label: Draft}
      published: {label: Published}
    properties:
      title: {type: string}
  control:
    label: Control
    id_prefix: "CTL-"
    properties:
      title: {type: string}
relations:
  implements:
    from: [policy]
    to: [control]
    scope: content
`

// faceEdgePolicy: drafter may delete the draft face and so its edges;
// gated-drafter holds the same grant, but writing an implements edge needs a
// permission it lacks; admin may delete every face and every control.
const faceEdgePolicy = `
role_relations:
  implements:
    requires_permission: manage-implements
roles:
  drafter:
    read: ["*"]
    permissions: [manage-implements]
    delete: ["policy@draft"]
  gated-drafter:
    read: ["*"]
    delete: ["policy@draft"]
  admin:
    read: ["*"]
    permissions: [manage-implements]
    delete: ["policy@draft", "policy@published", "control"]
assignments:
  drafter: drafter
  gated-drafter: gated-drafter
  admin: admin
`

type faceEdgeFixture struct {
	st  store.Store
	mgr *entitymanager.Manager
	aud *audit.Memory
}

// newFaceEdgeFixture seeds POL-1 at both faces, CTL-1 and CTL-2, and the
// edges POL-1@draft implements CTL-1 and POL-1@published implements CTL-2.
func newFaceEdgeFixture(t *testing.T, b concBackend) faceEdgeFixture {
	t.Helper()
	st := b.open(t)
	meta, err := metamodel.Parse([]byte(faceEdgeMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	p, err := acl.LoadPolicyBytes([]byte(faceEdgePolicy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	aud := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL: d, Transitions: statemachine.EmptySet(),
		FieldGate:       entitymanager.AllowAllFieldGate{},
		CopyReadGate:    entitymanager.AllowAllCopyReadGate{},
		CopyVisibility:  allowAllCopyVisibility(t, st),
		VersionRecorder: &fakeRecorder{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		policyFace("draft"),
		policyFace("published"),
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "Badge"}},
		{ID: "CTL-2", Type: "control", Properties: map[string]any{"title": "Clean desk"}},
	} {
		if cErr := st.CreateEntity(ctx, e); cErr != nil {
			t.Fatalf("seed %s@%q: %v", e.ID, e.Face, cErr)
		}
	}
	for face, to := range map[entity.Face]string{"draft": "CTL-1", "published": "CTL-2"} {
		if _, rErr := st.CreateRelation(ctx, "POL-1", "implements", to,
			&store.RelationData{FromFace: face}); rErr != nil {
			t.Fatalf("seed POL-1@%s implements %s: %v", face, to, rErr)
		}
	}
	return faceEdgeFixture{st: st, mgr: mgr, aud: aud}
}

// edgesFrom lists the implements edges still tailed at POL-1's face.
func (f faceEdgeFixture) edgesFrom(t *testing.T, face entity.Face) []string {
	t.Helper()
	var out []string
	for r, err := range f.st.ListRelations(context.Background(), store.RelationQuery{
		EntityID: "POL-1", Direction: store.DirectionOutgoing, FromFace: &face,
	}) {
		if err != nil {
			t.Fatalf("ListRelations POL-1@%s: %v", face, err)
		}
		out = append(out, r.To)
	}
	return out
}

func (f faceEdgeFixture) deleteRecords() int {
	n := 0
	for _, r := range f.aud.Records() {
		if r.Op == audit.OpDeleteEntity || r.Op == audit.OpDeleteRelation {
			n++
		}
	}
	return n
}

// Deleting a face resolves each edge's source type from the edge's own tail
// (BUG-58BL9I). Before the fix the source was read at the zero face, which a
// faced type does not store, so the check saw type "" and refused.
func TestDeleteEntityFace_ContentEdgeAuthorizedByItsTail(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)

			res, err := f.mgr.DeleteEntityFace(asUser("drafter"), "POL-1", "draft")
			if err != nil {
				t.Fatalf("DeleteEntityFace POL-1@draft as drafter: %v", err)
			}
			if len(res.DeletedRelations) != 1 || res.DeletedRelations[0].To != "CTL-1" {
				t.Errorf("DeletedRelations = %+v, want the draft's edge to CTL-1", res.DeletedRelations)
			}
			if _, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "draft"}); !errors.Is(gErr, store.ErrNotFound) {
				t.Errorf("POL-1@draft after delete: err = %v, want ErrNotFound", gErr)
			}
			if _, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "published"}); gErr != nil {
				t.Errorf("POL-1@published must survive: %v", gErr)
			}
			if got := f.edgesFrom(t, "published"); len(got) != 1 || got[0] != "CTL-2" {
				t.Errorf("published edges = %v, want [CTL-2]", got)
			}
		})
	}
}

// A principal allowed to delete the face but not its edges is refused before
// anything is written, and the refusal names the relation, not type "".
func TestDeleteEntityFace_ContentEdgeDeniedWritesNothing(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)

			_, err := f.mgr.DeleteEntityFace(asUser("gated-drafter"), "POL-1", "draft")

			var forbidden *acl.ForbiddenError
			if !errors.As(err, &forbidden) {
				t.Fatalf("DeleteEntityFace as gated-drafter = %v, want *acl.ForbiddenError", err)
			}
			if forbidden.Decision.RuleKind != "delegate-permission" {
				t.Errorf("denied by %q (%s), want the implements permission gate",
					forbidden.Decision.RuleKind, forbidden.Decision.Reason)
			}
			if _, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "draft"}); gErr != nil {
				t.Errorf("POL-1@draft must survive a denied delete: %v", gErr)
			}
			if got := f.edgesFrom(t, "draft"); len(got) != 1 {
				t.Errorf("draft edges after a denied delete = %v, want [CTL-1]", got)
			}
			if n := f.deleteRecords(); n != 0 {
				t.Errorf("a denied delete wrote %d delete audit records, want 0", n)
			}
		})
	}
}

// The family delete and a faceless cascade delete go through the same
// cascade check: each content edge is authorized at its own tail face, and a
// faceless source resolves as before.
func TestDelete_CascadeResolvesEdgeSourceType(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name+"/family", func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)
			res, err := f.mgr.DeleteEntity(asUser("admin"), "POL-1", true)
			if err != nil {
				t.Fatalf("family DeleteEntity POL-1 as admin: %v", err)
			}
			if len(res.DeletedEntities) != 2 || len(res.DeletedRelations) != 2 {
				t.Errorf("deleted %d faces and %d edges, want 2 and 2",
					len(res.DeletedEntities), len(res.DeletedRelations))
			}
		})
		t.Run(b.name+"/faceless-target", func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)
			res, err := f.mgr.DeleteEntity(asUser("admin"), "CTL-1", true)
			if err != nil {
				t.Fatalf("DeleteEntity CTL-1 as admin: %v", err)
			}
			if len(res.DeletedRelations) != 1 || res.DeletedRelations[0].FromFace != "draft" {
				t.Errorf("DeletedRelations = %+v, want the draft's edge", res.DeletedRelations)
			}
		})
	}
}

// The tail read falls back to any face of the family when the tail row is not
// there, and a source with no rows at all still fails closed. Each case
// deletes a faceless control whose one incoming edge has that source.
func TestDelete_CascadeSourceFallback(t *testing.T) {
	for _, b := range concBackends {
		for _, tc := range []struct {
			name      string
			seed      func(t *testing.T, st store.Store)
			from      string
			tail      entity.Face
			wantAllow bool
		}{
			{
				// POL-2 exists only at published; the edge names its draft.
				name: "tail face gone, family present",
				seed: func(t *testing.T, st store.Store) {
					t.Helper()
					p := policyFace("published")
					p.ID = "POL-2"
					if err := st.CreateEntity(context.Background(), p); err != nil {
						t.Fatalf("seed POL-2@published: %v", err)
					}
				},
				from: "POL-2", tail: "draft", wantAllow: true,
			},
			{
				name: "source has no rows",
				seed: func(*testing.T, store.Store) {},
				from: "POL-9", tail: "draft", wantAllow: false,
			},
		} {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				f := newFaceEdgeFixture(t, b)
				ctx := context.Background()
				tc.seed(t, f.st)
				ctl := &entity.Entity{ID: "CTL-9", Type: "control", Properties: map[string]any{"title": "Target"}}
				if err := f.st.CreateEntity(ctx, ctl); err != nil {
					t.Fatalf("seed CTL-9: %v", err)
				}
				if _, err := f.st.CreateRelation(ctx, tc.from, "implements", ctl.ID,
					&store.RelationData{FromFace: tc.tail}); err != nil {
					t.Skipf("%s refuses the seed edge: %v", b.name, err)
				}

				_, err := f.mgr.DeleteEntity(asUser("admin"), ctl.ID, true)

				var forbidden *acl.ForbiddenError
				switch {
				case tc.wantAllow && err != nil:
					t.Fatalf("DeleteEntity %s as admin: %v", ctl.ID, err)
				case !tc.wantAllow && !errors.As(err, &forbidden):
					t.Fatalf("DeleteEntity %s as admin = %v, want *acl.ForbiddenError", ctl.ID, err)
				}
				_, gErr := f.st.GetEntity(ctx, ctl.Ref())
				if gone := errors.Is(gErr, store.ErrNotFound); gone != tc.wantAllow {
					t.Errorf("%s gone = %v, want %v (err %v)", ctl.ID, gone, tc.wantAllow, gErr)
				}
			})
		}
	}
}

// failingTailStore fails every family read inside a transaction, so the
// cascade check cannot resolve an edge's source.
type failingTailStore struct{ store.Store }

func (s failingTailStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return s.Store.Tx(ctx, func(tx store.Store) error { return fn(failingTailTx{tx}) })
}

type failingTailTx struct{ store.Store }

var errTailRead = errors.New("tail read failed")

// ListEntities fails: an edge's source is resolved as a family (BUG-J3PBFN).
// The face row itself still reads, so the failure is the tail's.
func (failingTailTx) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) { yield(nil, errTailRead) }
}

// A store error while resolving an edge's source aborts the delete with that
// error. It is not reported as a denial, and nothing is removed.
func TestDeleteEntityFace_SourceReadErrorAborts(t *testing.T) {
	f := newFaceEdgeFixture(t, concBackends[0])
	meta, err := metamodel.Parse([]byte(faceEdgeMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: failingTailStore{f.st}, Meta: meta, Templater: nopTemplater{}, Audit: f.aud,
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	_, err = mgr.DeleteEntityFace(asUser("drafter"), "POL-1", "draft")

	if !errors.Is(err, errTailRead) {
		t.Fatalf("DeleteEntityFace = %v, want the tail read error", err)
	}
	if _, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "draft"}); gErr != nil {
		t.Errorf("POL-1@draft must survive an aborted delete: %v", gErr)
	}
	if got := f.edgesFrom(t, "draft"); len(got) != 1 {
		t.Errorf("draft edges after an aborted delete = %v, want [CTL-1]", got)
	}
}
