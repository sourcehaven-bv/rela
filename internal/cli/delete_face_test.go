package cli

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const facedCLIMetaYAML = `version: "1.0"
entities:
  policy:
    label: Policy
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

// facedCLIServices seeds POL-1 at draft and published, CTL-1, and an
// implements edge from each face to CTL-1.
func facedCLIServices(t *testing.T) *writeServices {
	t.Helper()
	meta, err := metamodel.Parse([]byte(facedCLIMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	b, err := newCLIBundles(appbuildtest.New(meta))
	if err != nil {
		t.Fatalf("build cli services: %v", err)
	}
	ctx := context.Background()
	st := b.write.Store
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "d"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "p"}},
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "c"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	for _, face := range []entity.Face{"draft", "published"} {
		if _, err := st.CreateRelation(ctx, "POL-1", "implements", "CTL-1",
			&store.RelationData{FromFace: face}); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	}
	return b.write
}

func facesOf(t *testing.T, st store.Store, id string) map[entity.Face]bool {
	t.Helper()
	faces := map[entity.Face]bool{}
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{IDs: []string{id}, Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		faces[e.Face] = true
	}
	return faces
}

// `rela delete` finds a faced entity by its family, and `ID@face` deletes
// only that face and its edges (BUG-J3PBFN).
func TestDeleteCmd_FacedEntity(t *testing.T) {
	ctx := context.Background()
	svc := facedCLIServices(t)
	withOutput(t, output.FormatTable)

	if err := (&DeleteCmd{ID: "POL-1@draft", Force: true, Cascade: true}).Run(ctx, svc); err != nil {
		t.Fatalf("delete POL-1@draft: %v", err)
	}
	if got := facesOf(t, svc.Store, "POL-1"); len(got) != 1 || !got["published"] {
		t.Fatalf("faces after the face delete = %v, want [published]", got)
	}
	n, err := svc.Store.CountRelations(ctx, store.RelationQuery{From: "POL-1"})
	if err != nil || n != 1 {
		t.Fatalf("edges after the face delete = %d (%v), want the published one", n, err)
	}

	if err := (&DeleteCmd{ID: "POL-1", Force: true, Cascade: true}).Run(ctx, svc); err != nil {
		t.Fatalf("delete POL-1: %v", err)
	}
	if got := facesOf(t, svc.Store, "POL-1"); len(got) != 0 {
		t.Fatalf("faces after the family delete = %v, want none", got)
	}
}

// --cascade guards the family delete only: the edges tailed at a face are its
// content, so a face delete takes them without the flag.
func TestDeleteCmd_CascadeGuardsTheFamilyOnly(t *testing.T) {
	ctx := context.Background()
	svc := facedCLIServices(t)
	withOutput(t, output.FormatTable)

	err := (&DeleteCmd{ID: "POL-1", Force: true}).Run(ctx, svc)
	if err == nil || err.Error() != "entity POL-1 has 2 relation(s); use --cascade to delete them too" {
		t.Fatalf("family delete err = %v, want the two-edge refusal", err)
	}

	if err = (&DeleteCmd{ID: "POL-1@draft", Force: true}).Run(ctx, svc); err != nil {
		t.Fatalf("delete POL-1@draft: %v", err)
	}
	draft := entity.Face("draft")
	n, err := svc.Store.CountRelations(ctx, store.RelationQuery{From: "POL-1", FromFace: &draft})
	if err != nil || n != 0 {
		t.Fatalf("draft edges after the face delete = %d (%v), want none", n, err)
	}
}

// `rela unlink POL-1@published ...` removes the edge on that tail and leaves
// the draft's (BUG-J3PBFN).
func TestUnlinkCmd_FaceTail(t *testing.T) {
	ctx := context.Background()
	svc := facedCLIServices(t)
	withOutput(t, output.FormatTable)
	if err := (&UnlinkCmd{From: "POL-1@published", Relation: "implements", To: "CTL-1"}).Run(ctx, svc); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	for face, want := range map[entity.Face]int{"draft": 1, "published": 0} {
		n, err := svc.Store.CountRelations(ctx, store.RelationQuery{From: "POL-1", FromFace: &face})
		if err != nil || n != want {
			t.Errorf("edges on %s = %d (%v), want %d", face, n, err, want)
		}
	}
	// The bare id names the identity tail, which holds no edge here.
	if err := (&UnlinkCmd{From: "POL-1", Relation: "implements", To: "CTL-1"}).Run(ctx, svc); err == nil {
		t.Error("unlink of the identity tail succeeded, want relation not found")
	}
}
