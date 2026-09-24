package visibility_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// GateTraversal authorizes for the ctx principal: the same hop is denied to
// a principal who cannot read the far type and allowed to one who can.
func TestDeclarativeGate_GateTraversalUsesCtxPrincipal(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{{ID: "alice", Type: "user"}, {ID: "bob", Type: "user"}} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"full":   {Read: []string{"ticket", "concept"}},
			"narrow": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "full", "bob": "narrow"},
	}, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		t.Fatal(err)
	}
	hop := acl.TraversalHop{RelationTypes: []string{"caused-by"}, EntityType: "concept"}
	as := func(user string) context.Context {
		return principal.With(ctx, principal.Principal{User: user, Tool: principal.ToolDataEntry})
	}

	if _, err := gate.GateTraversal(as("alice"), "ticket", hop); err != nil {
		t.Fatalf("alice: %v", err)
	}
	if _, err := gate.GateTraversal(as("bob"), "ticket", hop); !errors.Is(err, acl.ErrTraversalDenied) {
		t.Fatalf("bob: err = %v, want ErrTraversalDenied", err)
	}
}
