package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// lastFaceFixture is the face-edge fixture reduced to POL-1@draft, with an
// inbound edge CTL-2 covers POL-1. The draft is then the family's last face,
// so deleting it removes the entity and every incident edge (RR-2466U1).
func lastFaceFixture(t *testing.T, b concBackend) faceEdgeFixture {
	t.Helper()
	f := newFaceEdgeFixture(t, b)
	ctx := context.Background()
	if _, err := f.st.DeleteFace(ctx, entity.Ref{ID: "POL-1", Face: "published"}); err != nil {
		t.Fatalf("seed: delete POL-1@published: %v", err)
	}
	if _, err := f.st.CreateRelation(ctx, "CTL-2", "covers", "POL-1", nil); err != nil {
		t.Fatalf("seed CTL-2 covers POL-1: %v", err)
	}
	return f
}

func (f faceEdgeFixture) inboundCovers(t *testing.T) int {
	t.Helper()
	n := 0
	for _, err := range f.st.ListRelations(context.Background(), store.RelationQuery{
		EntityID: "POL-1", Direction: store.DirectionIncoming,
	}) {
		if err != nil {
			t.Fatalf("ListRelations into POL-1: %v", err)
		}
		n++
	}
	return n
}

// The last face takes its inbound edges with it, and each is authorized like
// a family-delete cascade: a principal who may delete the face but not the
// inbound edge is refused, and nothing is written.
func TestDeleteEntityFace_LastFaceAuthorizesInboundEdges(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := lastFaceFixture(t, b)

			_, err := f.mgr.DeleteEntityFace(asUser("drafter"), "POL-1", "draft", true)
			var forbidden *acl.ForbiddenError
			if !errors.As(err, &forbidden) {
				t.Fatalf("DeleteEntityFace as drafter = %v, want *acl.ForbiddenError for the inbound edge", err)
			}
			if _, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "draft"}); gErr != nil {
				t.Errorf("POL-1@draft must survive a denied delete: %v", gErr)
			}
			if n := f.inboundCovers(t); n != 1 {
				t.Errorf("inbound edges after a denied delete = %d, want 1", n)
			}
			if n := f.deleteRecords(); n != 0 {
				t.Errorf("a denied delete wrote %d delete audit records, want 0", n)
			}
		})
	}
}

func TestDeleteEntityFace_LastFaceRemovesInboundEdges(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := lastFaceFixture(t, b)

			res, err := f.mgr.DeleteEntityFace(asUser("admin"), "POL-1", "draft", true)
			if err != nil {
				t.Fatalf("DeleteEntityFace as admin: %v", err)
			}
			if n := f.inboundCovers(t); n != 0 {
				t.Errorf("inbound edges after the last face went = %d, want 0", n)
			}
			var covers, implements int
			for _, r := range res.DeletedRelations {
				switch r.Type {
				case "covers":
					covers++
				case "implements":
					implements++
				}
			}
			if covers != 1 || implements != 1 {
				t.Errorf("DeletedRelations = %+v, want the inbound covers and the draft's implements", res.DeletedRelations)
			}
		})
	}
}

// A face that is not the last leaves inbound edges alone: they name the
// entity, which survives.
func TestDeleteEntityFace_NotLastKeepsInboundEdges(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)
			if _, err := f.st.CreateRelation(context.Background(), "CTL-2", "covers", "POL-1", nil); err != nil {
				t.Fatalf("seed CTL-2 covers POL-1: %v", err)
			}
			// drafter cannot delete a covers edge, so success also shows the
			// inbound edge was not part of the authorized cascade.
			if _, err := f.mgr.DeleteEntityFace(asUser("drafter"), "POL-1", "draft", true); err != nil {
				t.Fatalf("DeleteEntityFace as drafter: %v", err)
			}
			if n := f.inboundCovers(t); n != 1 {
				t.Errorf("inbound edges = %d, want 1", n)
			}
		})
	}
}

// Without cascade, the last face with edges is refused like a family delete,
// and nothing is written.
func TestDeleteEntityFace_LastFaceNeedsCascade(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := lastFaceFixture(t, b)
			_, err := f.mgr.DeleteEntityFace(asUser("admin"), "POL-1", "draft", false)
			if !errors.Is(err, entitymanager.ErrHasRelations) {
				t.Fatalf("DeleteEntityFace without cascade = %v, want ErrHasRelations", err)
			}
			if _, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "draft"}); gErr != nil {
				t.Errorf("POL-1@draft must survive: %v", gErr)
			}
			if n := f.deleteRecords(); n != 0 {
				t.Errorf("a refused delete wrote %d delete audit records, want 0", n)
			}
		})
	}
}

// A face that is not the last takes its own edges without cascade.
func TestDeleteEntityFace_NotLastIgnoresCascade(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFaceEdgeFixture(t, b)
			if _, err := f.mgr.DeleteEntityFace(asUser("drafter"), "POL-1", "draft", false); err != nil {
				t.Fatalf("DeleteEntityFace without cascade: %v", err)
			}
		})
	}
}
