package acl

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// RR-QUXMAF (stage-2 design section 12): a gated hop reads its endpoint in
// the REQUEST's world, whatever selection the query running it carries. The
// concept's draft face satisfies the hop and its default face does not, so a
// default-world request must not match, even from an AllFaces query that
// would otherwise test every face of the endpoint.
func TestGateTraversal_AllFacesQueryCannotSatisfyHopOutsideRequestWorld(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	draft, err := entity.ParseFace("draft")
	if err != nil {
		t.Fatal(err)
	}
	mk := func(id, typ string, face entity.Face, status string) {
		t.Helper()
		e := entity.New(id, typ)
		e.Face = face
		if status != "" {
			e.SetString("status", status)
		}
		if cerr := st.CreateEntity(ctx, e); cerr != nil {
			t.Fatal(cerr)
		}
	}
	mk("alice", "user", "", "")
	mk("TKT-1", "ticket", "", "")
	mk("CON-1", "concept", "", "closed")
	mk("CON-1", "concept", draft, "open")
	if _, rerr := st.CreateRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "caused-by", To: "CON-1"}, nil); rerr != nil {
		t.Fatal(rerr)
	}
	d, err := NewDeclarative(&Policy{
		Roles:       map[string]RoleDef{"reader": {Read: []string{"ticket", "concept"}}},
		Assignments: map[string]string{"alice": "reader"},
	}, NewStoreGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	hop := TraversalHop{
		RelationTypes: []string{"caused-by"},
		EntityType:    "concept",
		Props:         []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
	}
	draftWorld := store.NewWorldScope(map[string]store.TypeResolution{
		"concept": {Chain: []entity.Face{draft}, Fallback: store.FallbackDefaultState},
	})

	matches := func(t *testing.T, world store.WorldScope, sel store.FaceSelection) bool {
		t.Helper()
		pred, gerr := requestFor(t, d, "alice").GateTraversal(ctx, "ticket", world, hop)
		if gerr != nil {
			t.Fatalf("GateTraversal: %v", gerr)
		}
		ids, gerr := st.MatchingIDs(ctx, store.GraphQuery{EntityType: "ticket", Faces: sel, HasOutbound: pred},
			[]string{"TKT-1"})
		if gerr != nil {
			t.Fatalf("MatchingIDs: %v", gerr)
		}
		return ids["TKT-1"]
	}

	for _, tc := range []struct {
		name  string
		world store.WorldScope
		sel   store.FaceSelection
		want  bool
	}{
		{"default world, AllFaces query", store.DefaultWorld(), store.AllFaces(), false},
		{"default world, AtFaces(draft) query", store.DefaultWorld(), store.AtFaces(draft), false},
		{"default world, InWorld query", store.DefaultWorld(), store.InWorld(store.DefaultWorld()), false},
		{"draft world, AllFaces query", draftWorld, store.AllFaces(), true},
		{"draft world, default-world query", draftWorld, store.InWorld(store.DefaultWorld()), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matches(t, tc.world, tc.sel); got != tc.want {
				t.Fatalf("TKT-1 matched = %v, want %v", got, tc.want)
			}
		})
	}

	// Control: the same hop with its endpoint selection cleared inherits the
	// AllFaces query and matches through the draft face. This is the
	// widening the stamp removes; without it the cases above prove nothing.
	pred, err := requestFor(t, d, "alice").GateTraversal(ctx, "ticket", store.DefaultWorld(), hop)
	if err != nil {
		t.Fatal(err)
	}
	pred.EndpointMatch.Faces = store.FaceSelection{}
	ids, err := st.MatchingIDs(ctx, store.GraphQuery{EntityType: "ticket", Faces: store.AllFaces(), HasOutbound: pred},
		[]string{"TKT-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !ids["TKT-1"] {
		t.Fatal("control: an unstamped endpoint under AllFaces should match through the draft face")
	}
}
