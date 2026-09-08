package acl

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// A relation-conferred role grants what IT declares, for the entities ITS
// relation reaches — not the union of every conferring role's faces. Before
// the per-relation branches, a principal who merely REVIEWED an entity passed
// the composed query through the `reviews` edge and received the owner's
// faces too, reading a bare (draft) row the reviewer role never granted.
func TestReadQuery_ConferredRolesKeepTheirOwnFaces(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Properties: map[string]any{"title": "bare"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "published"}},
		{ID: "bob", Type: "user"}, {ID: "alice", Type: "user"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range [][3]string{{"bob", "owns", "POL-1"}, {"alice", "reviews", "POL-1"}} {
		if _, err := st.CreateRelation(ctx, r[0], r[1], r[2], nil); err != nil {
			t.Fatal(err)
		}
	}
	p := &Policy{
		Roles: map[string]RoleDef{
			"author":   {Read: []string{"policy"}},
			"reviewer": {Read: []string{"policy@published"}},
		},
		RoleRelations: map[string]RoleRelationDef{
			"owns":    {Confers: "author"},
			"reviews": {Confers: "reviewer"},
		},
	}
	d, err := NewDeclarative(p, NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	published := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published"}, Fallback: store.FallbackExclude},
	})

	reads := func(t *testing.T, user string, world store.WorldScope) bool {
		t.Helper()
		req, err := d.ForPrincipal(principal.Principal{User: user, Tool: principal.ToolDataEntry})
		if err != nil {
			t.Fatalf("ForPrincipal: %v", err)
		}
		rqr := req.ReadQuery(ctx, "policy")
		if rqr.Query == nil {
			t.Fatalf("%s: want a composed query, got %+v", user, rqr)
		}
		q := *rqr.Query
		q.World = world
		q.FaceIn = rqr.Faces
		m, err := st.MatchingIDs(ctx, q, []string{"POL-1"})
		if err != nil {
			t.Fatal(err)
		}
		return m["POL-1"]
	}

	if !reads(t, "bob", store.DefaultWorld()) {
		t.Error("the owner's role reads every face, so bob reads the bare row")
	}
	if reads(t, "alice", store.DefaultWorld()) {
		t.Error("the reviewer's role grants published ONLY: alice must not read the bare " +
			"row through the union of the owner's faces")
	}
	if !reads(t, "alice", published) {
		t.Error("alice reads the published prime through her own relation")
	}
}
