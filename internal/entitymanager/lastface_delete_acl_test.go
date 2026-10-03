package entitymanager_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	if _, err := f.st.CreateRelation(ctx, entity.RelationKey{From: "CTL-2", Type: "covers", To: "POL-1"}, nil); err != nil {
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
			if _, err := f.st.CreateRelation(context.Background(), entity.RelationKey{From: "CTL-2", Type: "covers", To: "POL-1"}, nil); err != nil {
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

// A caller who cannot read the published face observes the same delete of
// the draft whether or not a published face exists. "Last face" is judged
// from the faces the caller may read, so the cascade opt-in and the
// authorization of the inbound edge apply in both worlds, and a refusal does
// not confirm a hidden sibling's absence. With a hidden sibling the store
// keeps the entity, so the hidden face, its edge and the inbound edge
// survive.
func TestDeleteEntityFace_HiddenSiblingIsNoOracle(t *testing.T) {
	for _, b := range concBackends {
		for _, tc := range []struct {
			name    string
			user    string
			cascade bool
			want    func(error) bool
		}{
			{"no cascade", "blind-admin", false, func(err error) bool { return errors.Is(err, entitymanager.ErrHasRelations) }},
			{"inbound edge denied", "blind-drafter", true, func(err error) bool {
				var forbidden *acl.ForbiddenError
				return errors.As(err, &forbidden)
			}},
			{"allowed", "blind-admin", true, func(err error) bool { return err == nil }},
		} {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				outcomes := map[bool]string{}
				for _, hidden := range []bool{false, true} {
					f := lastFaceFixture(t, b)
					if hidden {
						if err := f.st.CreateEntity(context.Background(), policyFace("published")); err != nil {
							t.Fatalf("seed POL-1@published: %v", err)
						}
					}
					_, err := f.mgr.DeleteEntityFace(asUser(tc.user), "POL-1", "draft", tc.cascade)
					if !tc.want(err) {
						t.Fatalf("hidden=%v: DeleteEntityFace as %s = %v", hidden, tc.user, err)
					}
					outcomes[hidden] = fmt.Sprint(err)
					if strings.Contains(outcomes[hidden], "published") {
						t.Errorf("hidden=%v: outcome %q names the hidden face", hidden, outcomes[hidden])
					}
					if !hidden {
						continue
					}
					if _, gErr := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: "published"}); gErr != nil {
						t.Errorf("the hidden published face must survive: %v", gErr)
					}
					if n := f.inboundCovers(t); n != 1 {
						t.Errorf("inbound edges with a hidden sibling = %d, want 1: the entity survives", n)
					}
				}
				if outcomes[false] != outcomes[true] {
					t.Errorf("without a hidden sibling = %q, with one = %q; want the same", outcomes[false], outcomes[true])
				}
			})
		}
	}
}
