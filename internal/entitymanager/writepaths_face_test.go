package entitymanager_test

import (
	"context"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// cascadeFaceMetaYAML is a faced policy whose automations write
// content-scoped edges from the trigger face (BUG-J3PBFN).
const cascadeFaceMetaYAML = `version: "1.0"
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
      status: {type: string}
  checklist:
    label: Checklist
    id_prefix: "CL-"
    properties:
      title: {type: string}
  control:
    label: Control
    id_prefix: "CTL-"
    properties:
      title: {type: string}
relations:
  has-checklist:
    from: [policy]
    to: [checklist]
    scope: content
  implements:
    from: [policy]
    to: [control]
    scope: content
`

// newCascadeFaceManager wires a manager over st whose automations, on a
// policy face reaching status "review", create a checklist (replacing an
// existing one) and link CTL-1.
func newCascadeFaceManager(t *testing.T, st store.Store) (*entitymanager.Manager, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(cascadeFaceMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	engine := automation.NewEngine([]automation.Automation{{
		Name: "checklist-on-review",
		On:   automation.Trigger{Entity: []string{"policy"}, Property: "status", Becomes: "review"},
		Do: []automation.Action{
			{CreateEntity: &automation.CreateEntityAction{
				Type: "checklist", Relation: "has-checklist", IfExists: automation.IfExistsReplace,
			}},
			{CreateRelation: &automation.CreateRelationAction{Relation: "implements", To: "CTL-1"}},
		},
	}})
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	if err != nil {
		t.Fatalf("autocascade.New: %v", err)
	}
	aud := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		Automations: engine, Cascade: runner,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, aud
}

// setPolicyStatus updates POL-1 at face to status.
func setPolicyStatus(t *testing.T, mgr *entitymanager.Manager, st store.Store, face entity.Face, status string) {
	t.Helper()
	ctx := context.Background()
	cur, err := st.GetEntity(ctx, entity.Ref{ID: "POL-1", Face: face})
	if err != nil {
		t.Fatalf("read POL-1@%s: %v", face, err)
	}
	next := cur.Clone()
	next.Properties["status"] = status
	if _, err := mgr.UpdateEntity(ctx, next); err != nil {
		t.Fatalf("update POL-1@%s: %v", face, err)
	}
}

// outgoing lists POL-1's relType edges tailed at face, as target ids.
func outgoing(t *testing.T, st store.Store, face entity.Face, relType string) []string {
	t.Helper()
	var ids []string
	for rel, err := range st.ListRelations(context.Background(), store.RelationQuery{
		From: "POL-1", FromFace: &face, Type: relType,
	}) {
		if err != nil {
			t.Fatalf("ListRelations: %v", err)
		}
		ids = append(ids, rel.To)
	}
	return ids
}

// An automation fired by a face writes its content-scoped edges on that face,
// both the create_entity trigger relation and a create_relation action, and
// an if_exists: replace looks for the existing target on that face only.
func TestCascade_ContentEdgesKeepTheTriggerFace(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			st := b.open(t)
			mgr, _ := newCascadeFaceManager(t, st)
			ctx := context.Background()
			for _, e := range []*entity.Entity{
				policyFace("draft"), policyFace("published"),
				{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "Badge"}},
			} {
				if err := st.CreateEntity(ctx, e); err != nil {
					t.Fatalf("seed: %v", err)
				}
			}

			setPolicyStatus(t, mgr, st, "draft", "review")
			first := outgoing(t, st, "draft", "has-checklist")
			if len(first) != 1 {
				t.Fatalf("draft checklists after first review = %v, want one", first)
			}
			if got := outgoing(t, st, "draft", "implements"); len(got) != 1 || got[0] != "CTL-1" {
				t.Errorf("draft implements = %v, want [CTL-1]", got)
			}
			if got := outgoing(t, st, "", "has-checklist"); len(got) != 0 {
				t.Errorf("identity-tail checklists = %v, want none: the edge belongs to the draft", got)
			}

			// The published face finds no checklist on its own tail, so it
			// creates one and leaves the draft's alone.
			setPolicyStatus(t, mgr, st, "published", "review")
			if got := outgoing(t, st, "published", "has-checklist"); len(got) != 1 {
				t.Fatalf("published checklists = %v, want one", got)
			}
			if got := outgoing(t, st, "draft", "has-checklist"); len(got) != 1 || got[0] != first[0] {
				t.Fatalf("draft checklists after the published review = %v, want %v", got, first)
			}

			// A second review on the draft replaces the draft's checklist.
			setPolicyStatus(t, mgr, st, "draft", "open")
			setPolicyStatus(t, mgr, st, "draft", "review")
			second := outgoing(t, st, "draft", "has-checklist")
			if len(second) != 1 || second[0] == first[0] {
				t.Fatalf("draft checklists after the replace = %v, want one other than %v", second, first)
			}
			if _, err := st.GetEntity(ctx, entity.Ref{ID: first[0]}); err == nil {
				t.Errorf("replaced checklist %s still exists", first[0])
			}
		})
	}
}

// The cascade host's delete is the manager's family delete: every face goes,
// with one audit record and one version per face (BUG-J3PBFN). It used to
// read the bare row, which a faced type does not store, and so reported the
// family missing.
func TestCascadeHostDelete_DeletesEveryFace(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)
			meta, err := metamodel.Parse([]byte(faceEdgeMetaYAML))
			if err != nil {
				t.Fatalf("metamodel.Parse: %v", err)
			}
			rec := &fakeRecorder{}
			aud := audit.NewMemory()
			mgr, err := entitymanager.New(entitymanager.Deps{
				Store: f.st, Meta: meta, Templater: nopTemplater{}, Audit: aud,
				// Deny-all: the cascade was authorized by its trigger, so the
				// host's delete does not consult the ACL.
				ACL: acl.ReadOnlyACL{}, Transitions: statemachine.EmptySet(),
				FieldGate: entitymanager.AllowAllFieldGate{}, VersionRecorder: rec,
			})
			if err != nil {
				t.Fatalf("entitymanager.New: %v", err)
			}

			if err := entitymanager.CascadeHostDelete(context.Background(), mgr, "POL-1", true); err != nil {
				t.Fatalf("cascade host delete: %v", err)
			}

			if left := familyFacesOf(t, f.st, "POL-1"); len(left) != 0 {
				t.Errorf("faces left = %v, want none", left)
			}
			var entityDeletes, relDeletes, denied int
			faces := map[entity.Face]bool{}
			for _, r := range aud.Records() {
				// The cascaded relation deletes name the entity delete
				// instead, as every family delete labels them.
				if r.Op == audit.OpDeleteEntity && r.TriggeredBy != "automation" {
					t.Errorf("entity delete triggered_by = %q, want automation: the ctx named none", r.TriggeredBy)
				}
				switch r.Op {
				case audit.OpDeleteEntity:
					entityDeletes++
				case audit.OpDeleteRelation:
					relDeletes++
				case audit.OpDeniedWrite, audit.OpACLBypass:
					denied++
				}
			}
			for _, v := range rec.records {
				if v.Op == store.VersionOpDelete && v.EntityID == "POL-1" {
					faces[v.Face] = true
				}
			}
			if entityDeletes != 2 || relDeletes != 2 {
				t.Errorf("audit: %d entity and %d relation deletes, want 2 and 2", entityDeletes, relDeletes)
			}
			if denied != 0 {
				t.Errorf("audit: %d denied or bypass rows, want none for a cascade write", denied)
			}
			if !faces["draft"] || !faces["published"] || len(faces) != 2 {
				t.Errorf("delete versions by face = %v, want draft and published", faces)
			}
		})
	}
}

// familyFacesOf lists the stored faces of id.
func familyFacesOf(t *testing.T, st store.Store, id string) []entity.Face {
	t.Helper()
	var faces []entity.Face
	q := store.EntityQuery{IDs: []string{id}, Faces: store.AllFaces()}
	for e, err := range st.ListEntities(context.Background(), q) {
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		faces = append(faces, e.Face)
	}
	return faces
}

// updateBeforeTx applies an update to POL-1@draft just before the manager's
// transaction starts: the write a concurrent editor lands between the
// manager's early read and its delete.
type updateBeforeTx struct {
	store.Store
	once sync.Once
}

func (s *updateBeforeTx) Tx(ctx context.Context, fn func(store.Store) error) error {
	s.once.Do(func() {
		cur, err := s.GetEntity(ctx, entity.Ref{ID: "POL-1", Face: "draft"})
		if err != nil {
			panic(err)
		}
		next := cur.Clone()
		next.Properties["title"] = "edited concurrently"
		if err := s.UpdateEntity(ctx, next); err != nil {
			panic(err)
		}
	})
	return s.Store.Tx(ctx, fn)
}

// DeleteEntityFace records the row its transaction deletes, not the one it
// read before the transaction began (BUG-J3PBFN).
func TestDeleteEntityFace_RecordsTheRowReadInsideTheTx(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)
			meta, err := metamodel.Parse([]byte(faceEdgeMetaYAML))
			if err != nil {
				t.Fatalf("metamodel.Parse: %v", err)
			}
			rec := &fakeRecorder{}
			mgr, err := entitymanager.New(entitymanager.Deps{
				Store: &updateBeforeTx{Store: f.st}, Meta: meta, Templater: nopTemplater{},
				Audit: audit.Nop{}, ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
				FieldGate: entitymanager.AllowAllFieldGate{}, VersionRecorder: rec,
			})
			if err != nil {
				t.Fatalf("entitymanager.New: %v", err)
			}

			res, err := mgr.DeleteEntityFace(context.Background(), "POL-1", "draft")
			if err != nil {
				t.Fatalf("DeleteEntityFace: %v", err)
			}

			if got := res.DeletedEntities[0].Properties["title"]; got != "edited concurrently" {
				t.Errorf("result title = %v, want the in-transaction row's", got)
			}
			var found bool
			for _, v := range rec.records {
				if v.Op == store.VersionOpDelete && v.Face == "draft" {
					found = true
					if got := v.Properties["title"]; got != "edited concurrently" {
						t.Errorf("delete version title = %v, want the in-transaction row's", got)
					}
				}
			}
			if !found {
				t.Error("no delete version for POL-1@draft")
			}
		})
	}
}
