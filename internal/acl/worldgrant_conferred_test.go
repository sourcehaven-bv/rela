package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// A world is a GLOBAL lens, so a world grant on a relation-conferred role is
// honored globally: holding the role through any relation to any entity
// opens the world, and the per-entity and per-face gates decide what it
// shows. PermitsWorld used to walk assigned roles only, which made such a
// grant silently inert while the load-time refusal argued it was dangerous.
func TestPermitsWorld_ConferredRoleOpensTheWorldGlobally(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "page"}, {ID: "alice", Type: "user"}, {ID: "carol", Type: "user"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreateRelation(ctx, "alice", "owns", "POL-1", nil); err != nil {
		t.Fatal(err)
	}
	p := &acl.Policy{
		Roles: map[string]acl.RoleDef{"owner": {Read: []string{"page", "world:published"}}},
		RoleRelations: map[string]acl.RoleRelationDef{
			"owns": {Confers: "owner", RequiresPermission: "manage-owners"},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("policy must load: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	permits := func(user string) bool {
		t.Helper()
		req, err := d.ForPrincipal(principal.Principal{User: user, Tool: principal.ToolDataEntry})
		if err != nil {
			t.Fatalf("ForPrincipal: %v", err)
		}
		ok, err := req.PermitsWorld(ctx, "published")
		if err != nil {
			t.Fatalf("PermitsWorld: %v", err)
		}
		return ok
	}
	if !permits("alice") {
		t.Error("alice owns POL-1, so the owner role's world grant opens the world for her")
	}
	if permits("carol") {
		t.Error("carol owns nothing, holds no role, and must not read the world")
	}
}
