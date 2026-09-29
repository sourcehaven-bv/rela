package datamigration

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// faceCapture records the id@face of every entity version it is asked to
// write.
type faceCapture struct {
	entities []string
}

func (c *faceCapture) WriteVersion(_ context.Context, in store.VersionInput) error {
	c.entities = append(c.entities, entity.FormatStateRef(in.EntityID, in.Face))
	return nil
}

func (c *faceCapture) WriteRelationVersion(context.Context, store.RelationVersionInput) error {
	return nil
}

// TestDropEntities_FacedType pins that drop_entities drops an entity of a
// faced type, and captures every face first. A faced entity has no zero-face
// row (DEC-NPZICR), so a zero-face read found nothing, and the step skipped it
// as "already deleted": the rows survived a drop that reported success.
func TestDropEntities_FacedType(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	for _, face := range []entity.Face{"draft", "published"} {
		e := &entity.Entity{ID: "PER-F", Type: "person", Face: face, Properties: map[string]any{"name": "f"}}
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed PER-F@%s: %v", face, err)
		}
	}
	from := metaV1()
	to := metaV1()
	delete(to.Entities, "person")
	f := mustParse(t, testName("drop"), mustFileYAML(t, from, to, "  - drop_entities: {type: person}\n"))

	capture := &faceCapture{}
	r := newTestRunner(t, Deps{Store: st, Meta: to, Versions: capture})
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("apply: %v", err)
	}

	q := store.EntityQuery{IDs: []string{"PER-F"}, Faces: store.AllFaces()}
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			t.Fatal(err)
		}
		t.Errorf("PER-F@%s survived the drop", e.Face)
	}
	for _, want := range []string{"PER-F@draft", "PER-F@published"} {
		if !slices.Contains(capture.entities, want) {
			t.Errorf("no pre-delete capture of %s; captured %v", want, capture.entities)
		}
	}
}
