package dataentry

import (
	"context"
	"testing"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// facesSvc builds the minimal affordanceService computeFaces needs: a store to
// ask which faces exist, and a metamodel to know which are declared.
//
// Deliberately not a whole App. computeFaces reads exactly two collaborators,
// and a full fixture would let a future change quietly add a third without
// this test noticing it had grown a dependency.
func facesSvc(t *testing.T) (affordanceService, *memstore.MemStore) {
	t.Helper()
	st := memstore.New()
	m := worldsMeta()
	return affordanceService{
		store: st,
		meta:  func() *metamodel.Metamodel { return m },
	}, st
}

func seedFaceInStore(ctx context.Context, t *testing.T, st *memstore.MemStore, id, typ string, p entityPkg.Face) {
	t.Helper()
	if err := st.CreateEntity(ctx, &entityPkg.Entity{
		ID: id, Type: typ, Face: p,
		Properties: map[string]any{"title": string(p) + " face"},
	}); err != nil {
		t.Fatalf("seed %s@%q: %v", id, p, err)
	}
}

// TestComputeFaces covers the four rules the "view the published face" /
// language-menu affordance rests on.
//
// The load-bearing one is EXISTENCE: a face NAME is config (declared in
// schema.yaml, public), but whether THIS entity has that face is data. Getting
// it wrong in the permissive direction offers a link to a face that is not
// there; in the restrictive direction it hides a face that is.
func TestComputeFaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("reports only faces the entity actually HAS", func(t *testing.T) {
		t.Parallel()
		svc, st := facesSvc(t)
		// POL-1 has both; POL-2 is draft-only. Same type, same declared
		// faces — so a result that merely echoed the metamodel would give
		// both entities the same answer, and this pins that it does not.
		seedFaceInStore(ctx, t, st, "POL-1", "policy", "")
		seedFaceInStore(ctx, t, st, "POL-1", "policy", "published")
		seedFaceInStore(ctx, t, st, "POL-2", "policy", "")

		got := svc.computeFaces(ctx, &entityPkg.Entity{ID: "POL-1", Type: "policy"})
		if len(got) != 1 || got[0].Face != "published" {
			t.Errorf("POL-1 has a published face; got %+v", got)
		}

		got = svc.computeFaces(ctx, &entityPkg.Entity{ID: "POL-2", Type: "policy"})
		if len(got) != 0 {
			t.Errorf("POL-2 is draft-only, so it has no OTHER face; got %+v", got)
		}
	})

	t.Run("excludes the face being served", func(t *testing.T) {
		t.Parallel()
		svc, st := facesSvc(t)
		seedFaceInStore(ctx, t, st, "POL-1", "policy", "")
		seedFaceInStore(ctx, t, st, "POL-1", "policy", "published")

		// Asked FROM the published face, the answer is the draft — not
		// published. This answers "where else can I go", and the face you are
		// on is not somewhere else.
		got := svc.computeFaces(ctx, &entityPkg.Entity{
			ID: "POL-1", Type: "policy", Face: "published",
		})
		if len(got) != 1 || got[0].Face != "" {
			t.Errorf("from published, the other face is the default one; got %+v", got)
		}
	})

	t.Run("lists every translation, sorted", func(t *testing.T) {
		t.Parallel()
		svc, st := facesSvc(t)
		seedFaceInStore(ctx, t, st, "POST-1", "blog-post", "")
		seedFaceInStore(ctx, t, st, "POST-1", "blog-post", "nl")
		seedFaceInStore(ctx, t, st, "POST-1", "blog-post", "fr")

		got := svc.computeFaces(ctx, &entityPkg.Entity{ID: "POST-1", Type: "blog-post"})
		if len(got) != 2 {
			t.Fatalf("both translations are offered; got %+v", got)
		}
		// Sorted, so the menu order does not depend on Go's map iteration —
		// otherwise the entries would shuffle between renders.
		if got[0].Label != "fr" || got[1].Label != "nl" {
			t.Errorf("faces must be sorted by label; got %q, %q", got[0].Label, got[1].Label)
		}
	})

	t.Run("returns empty, never nil, for a type with no faces", func(t *testing.T) {
		t.Parallel()
		svc, st := facesSvc(t)
		seedFaceInStore(ctx, t, st, "TKT-1", "ticket", "")

		got := svc.computeFaces(ctx, &entityPkg.Entity{ID: "TKT-1", Type: "ticket"})
		if got == nil {
			t.Fatal("must be non-nil: `_faces: []` is a real answer (this entity " +
				"has no other faces), and nil would omit the key, which reads " +
				"as 'the server could not tell you'")
		}
		if len(got) != 0 {
			t.Errorf("a faceless type has no other faces; got %+v", got)
		}
	})
}
