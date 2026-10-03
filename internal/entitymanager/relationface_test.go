package entitymanager_test

import (
	"context"
	"errors"
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

// relationFaceYAML pairs a FACED source type with both relation scopes, so
// the symmetric rule can be exercised in every combination that matters:
// content-scoped edges carry a face, identity-scoped edges never do.
const relationFaceYAML = `
entities:
  beleid:
    label: Beleid
    id_prefix: "POL-"
    id_type: manual
    faces:
      concept: {label: Concept}
      vastgesteld: {label: Vastgesteld}
    properties:
      title: {type: string}
  bron:
    label: Bron
    id_prefix: "SRC-"
    id_type: manual
    properties:
      title: {type: string}
relations:
  citeert:
    label: Citeert
    scope: content
    from: [beleid]
    to: [bron]
  geschreven-door:
    label: Geschreven door
    scope: identity
    from: [beleid]
    to: [bron]
  bron-verwijst:
    label: Bron verwijst
    scope: content
    from: [bron]
    to: [bron]
`

func relationFaceManager(t *testing.T) (*entitymanager.Manager, *memstore.MemStore) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(relationFaceYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

// seedRelationFaceGraph creates a faced source on both faces plus a faceless
// source and a target, so every case below has real endpoints.
func seedRelationFaceGraph(t *testing.T, mgr *entitymanager.Manager) {
	t.Helper()
	ctx := context.Background()
	for _, face := range []entity.Face{"concept", "vastgesteld"} {
		if _, err := mgr.CreateEntity(ctx, &entity.Entity{
			Type: "beleid", Properties: map[string]any{"title": "P"},
		}, entity.CreateOptions{ID: "POL-1", Face: face}); err != nil {
			t.Fatalf("seed beleid@%s: %v", face, err)
		}
	}
	if _, err := mgr.CreateEntity(ctx, &entity.Entity{
		Type: "bron", Properties: map[string]any{"title": "S"},
	}, entity.CreateOptions{ID: "SRC-1"}); err != nil {
		t.Fatalf("seed bron: %v", err)
	}
	if _, err := mgr.CreateEntity(ctx, &entity.Entity{
		Type: "bron", Properties: map[string]any{"title": "S2"},
	}, entity.CreateOptions{ID: "SRC-2"}); err != nil {
		t.Fatalf("seed bron 2: %v", err)
	}
}

// TestCreateRelation_FaceMustMatchScope is the rule RR-9LM7T7 found missing:
// before it, opts.FromFace reached the store with no metamodel consultation
// at all, so an identity-scoped edge could be filed at a face no reader ever
// queries — present in storage and invisible to every read.
func TestCreateRelation_FaceMustMatchScope(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		relType string
		to      string
		face    entity.Face
		wantErr error
	}{
		{
			name: "content scope on faced source accepts a declared face",
			from: "POL-1", relType: "citeert", to: "SRC-1",
			face: "concept", wantErr: nil,
		},
		{
			name: "content scope on faced source rejects an undeclared face",
			from: "POL-1", relType: "citeert", to: "SRC-1",
			face: "nope", wantErr: entitymanager.ErrFaceNotDeclared,
		},
		{
			// A zero tail would belong to no face. Every client resolves
			// the face before it calls the manager, so none needs this.
			name: "content scope on faced source rejects a zero face",
			from: "POL-1", relType: "citeert", to: "SRC-1",
			face: "", wantErr: entitymanager.ErrRelationFaceRequired,
		},
		{
			// The case that motivated the check. `geschreven-door` attaches
			// to the entity, so a tail names a row that does not exist.
			name: "identity scope rejects any face",
			from: "POL-1", relType: "geschreven-door", to: "SRC-1",
			face: "concept", wantErr: entitymanager.ErrFaceNotDeclared,
		},
		{
			name: "identity scope accepts the zero face",
			from: "POL-1", relType: "geschreven-door", to: "SRC-1",
			face: "", wantErr: nil,
		},
		{
			name: "content scope on a FACELESS source rejects a face",
			from: "SRC-1", relType: "bron-verwijst", to: "SRC-2",
			face: "concept", wantErr: entitymanager.ErrFaceNotDeclared,
		},
		{
			name: "content scope on a faceless source accepts the zero face",
			from: "SRC-1", relType: "bron-verwijst", to: "SRC-2",
			face: "", wantErr: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr, st := relationFaceManager(t)
			seedRelationFaceGraph(t, mgr)
			ctx := context.Background()

			_, err := mgr.CreateRelation(ctx, entity.RelationKey{From: tc.from, FromFace: tc.face, Type: tc.relType, To: tc.to},
				entity.RelationOptions{})

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("CreateRelation: unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateRelation error = %v, want %v", err, tc.wantErr)
			}
			// A refusal must also mean NO WRITE. An implementation that
			// validated after calling the store would satisfy the error
			// assertion above while having already filed the edge.
			for rel, lErr := range st.ListRelations(ctx, store.RelationQuery{}) {
				if lErr != nil {
					t.Fatalf("ListRelations: %v", lErr)
				}
				if rel.From == tc.from && rel.Type == tc.relType && rel.To == tc.to {
					t.Fatalf("refused relation was written anyway: %+v", rel)
				}
			}
		})
	}
}

// TestCreateRelation_FaceCheckPrecedesACL pins the ordering RR-3NNWNK turned
// on: the check must sit BELOW the ACL, because authorizeAndAudit returns
// early under bypassACL and would otherwise leave the elevated path with no
// validation between a Lua string and the store.
func TestCreateRelation_FaceCheckPrecedesACL(t *testing.T) {
	meta, err := metamodel.Parse([]byte(relationFaceYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	gate := &recordingACL{}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: gate, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	seedRelationFaceGraph(t, mgr)
	gate.relationCalls = 0

	_, err = mgr.CreateRelation(context.Background(), entity.RelationKey{From: "POL-1", FromFace: "concept", Type: "geschreven-door", To: "SRC-1"},
		entity.RelationOptions{})
	if !errors.Is(err, entitymanager.ErrFaceNotDeclared) {
		t.Fatalf("CreateRelation error = %v, want ErrFaceNotDeclared", err)
	}
	if gate.relationCalls != 0 {
		t.Errorf("ACL was consulted %d times for a face the metamodel rejects; "+
			"the check must run BEFORE the subject is built, so the elevated "+
			"path (which skips the ACL entirely) is still covered",
			gate.relationCalls)
	}
}

// recordingACL counts relation authorizations so a test can assert the face
// check ran first.
type recordingACL struct {
	relationCalls int
}

func (a *recordingACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if _, ok := req.Subject.(acl.RelationSubject); ok {
		a.relationCalls++
	}
	return acl.Decision{Allow: true}
}

// TestRelationWrites_RefuseMalformedTail pins the tail grammar: a tail that
// entity.ParseFace rejects is refused on create, update and delete alike,
// before any lookup. Delete checks only the grammar, so an edge filed under
// an older schema stays deletable.
func TestRelationWrites_RefuseMalformedTail(t *testing.T) {
	key := entity.RelationKey{From: "POL-1", FromFace: "Bad Face", Type: "citeert", To: "SRC-1"}
	tests := []struct {
		name  string
		write func(*entitymanager.Manager) error
	}{
		{"create", func(m *entitymanager.Manager) error {
			_, err := m.CreateRelation(context.Background(), key, entity.RelationOptions{})
			return err
		}},
		{"update", func(m *entitymanager.Manager) error {
			_, err := m.UpdateRelation(context.Background(), key, entity.RelationOptions{})
			return err
		}},
		{"delete", func(m *entitymanager.Manager) error {
			return m.DeleteRelation(context.Background(), key)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr, _ := relationFaceManager(t)
			seedRelationFaceGraph(t, mgr)
			if err := tc.write(mgr); !errors.Is(err, entitymanager.ErrFaceNotDeclared) {
				t.Fatalf("error = %v, want ErrFaceNotDeclared", err)
			}
		})
	}
}

// TestCreateRelation_RefusesTailOnUndeclaredSourceType pins that a source
// whose type the schema no longer declares has no faces, so a named tail
// names nothing.
func TestCreateRelation_RefusesTailOnUndeclaredSourceType(t *testing.T) {
	mgr, st := relationFaceManager(t)
	seedRelationFaceGraph(t, mgr)
	if err := st.CreateEntity(context.Background(), entity.New("OLD-1", "verouderd")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := mgr.CreateRelation(context.Background(),
		entity.RelationKey{From: "OLD-1", FromFace: "concept", Type: "citeert", To: "SRC-1"},
		entity.RelationOptions{})
	if !errors.Is(err, entitymanager.ErrFaceNotDeclared) {
		t.Fatalf("error = %v, want ErrFaceNotDeclared", err)
	}
}

// TestDeleteRelation_ByKeyRemovesOnlyThatTail pins that the key's tail picks
// the edge: deleting the concept-tailed edge leaves the zero-tailed one, and
// a zero-tailed content edge stored before faces were required can still be
// deleted. That edge is seeded through the store, since the manager now
// refuses to create it.
func TestDeleteRelation_ByKeyRemovesOnlyThatTail(t *testing.T) {
	mgr, st := relationFaceManager(t)
	seedRelationFaceGraph(t, mgr)
	ctx := context.Background()
	tailed := entity.RelationKey{From: "POL-1", FromFace: "concept", Type: "citeert", To: "SRC-1"}
	legacy := entity.RelationKey{From: "POL-1", FromFace: entity.ImplicitFace, Type: "citeert", To: "SRC-1"}
	if _, err := mgr.CreateRelation(ctx, tailed, entity.RelationOptions{}); err != nil {
		t.Fatalf("seed tailed: %v", err)
	}
	if _, err := st.CreateRelation(ctx, legacy, &store.RelationData{}); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if err := mgr.DeleteRelation(ctx, tailed); err != nil {
		t.Fatalf("DeleteRelation: %v", err)
	}
	if _, err := st.GetRelation(ctx, tailed); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("tailed edge: err = %v, want ErrNotFound", err)
	}
	if _, err := st.GetRelation(ctx, legacy); err != nil {
		t.Fatalf("zero-tailed edge must survive: %v", err)
	}
	if err := mgr.DeleteRelation(ctx, legacy); err != nil {
		t.Errorf("a zero-tailed content edge must stay deletable: %v", err)
	}
}
