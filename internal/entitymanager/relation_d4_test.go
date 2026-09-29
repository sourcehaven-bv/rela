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
)

const d4MetaYAML = faceEdgeMetaYAML + `  owns:
    from: [policy]
    to: [control]
`

// d4Policy: "bare" holds bare-type grants, which cover only the zero face;
// "drafter" holds the draft face; "editor" holds both faces.
const d4Policy = `
roles:
  bare:
    read: ["*"]
    create: ["policy"]
    delete: ["policy", "control"]
  drafter:
    read: ["*"]
    create: ["policy@draft"]
    delete: ["policy@draft", "control"]
  editor:
    read: ["*"]
    create: ["policy@draft", "policy@published"]
    delete: ["policy@draft", "policy@published", "control"]
assignments:
  bare: bare
  drafter: drafter
  editor: editor
`

// newD4Manager seeds POL-1 at both faces, CTL-1, and POL-1 owns CTL-1, an
// identity edge.
func newD4Manager(t *testing.T, b concBackend) (*entitymanager.Manager, store.Store) {
	t.Helper()
	st := b.open(t)
	meta, err := metamodel.Parse([]byte(d4MetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	p, err := acl.LoadPolicyBytes([]byte(d4Policy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
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
		policyFace("draft"), policyFace("published"),
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "Badge"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if _, err := st.CreateRelation(ctx, "POL-1", "owns", "CTL-1", nil); err != nil {
		t.Fatalf("seed owns: %v", err)
	}
	return mgr, st
}

// A zero-tailed edge from a faced source belongs to the whole family, for a
// content-scoped type as for an identity one (D4). A bare-type grant covers
// only the zero face, which a faced type does not store, so it is not enough.
func TestCreateRelation_ZeroTailOnFacedSourceNeedsEveryFace(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			for _, tc := range []struct {
				user  string
				allow bool
			}{{"bare", false}, {"drafter", false}, {"editor", true}} {
				t.Run(tc.user, func(t *testing.T) {
					mgr, _ := newD4Manager(t, b)
					_, err := mgr.CreateRelation(asUser(tc.user), "POL-1", "implements", "CTL-1",
						entity.RelationOptions{})
					if tc.allow && err != nil {
						t.Fatalf("CreateRelation: %v", err)
					}
					if !tc.allow && !errors.Is(err, acl.ErrForbidden) {
						t.Fatalf("CreateRelation err = %v, want ErrForbidden", err)
					}
				})
			}
		})
	}
}

// Deleting a target cascades its incoming identity edge from a faced source,
// and that edge's delete is authorized on every face of its source.
func TestDelete_CascadeIdentityEdgeFromFacedSourceNeedsEveryFace(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			for _, tc := range []struct {
				user  string
				allow bool
			}{{"bare", false}, {"drafter", false}, {"editor", true}} {
				t.Run(tc.user, func(t *testing.T) {
					mgr, st := newD4Manager(t, b)
					_, err := mgr.DeleteEntity(asUser(tc.user), "CTL-1", true)
					if tc.allow && err != nil {
						t.Fatalf("DeleteEntity: %v", err)
					}
					if tc.allow {
						return
					}
					if !errors.Is(err, acl.ErrForbidden) {
						t.Fatalf("DeleteEntity err = %v, want ErrForbidden", err)
					}
					if _, gErr := st.GetEntity(context.Background(), entity.Ref{ID: "CTL-1"}); gErr != nil {
						t.Errorf("CTL-1 after a denied delete: %v", gErr)
					}
				})
			}
		})
	}
}
