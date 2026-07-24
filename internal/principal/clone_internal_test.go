package principal

import (
	"context"
	"testing"
)

// In-package on purpose. The aliasing this guards against is only observable
// on the UNEXPORTED roles field: Roles() clones on every read, so an external
// test comparing &a.Roles()[0] to &b.Roles()[0] compares two fresh allocations
// and can never fail — it would pass with Clone deleted. That tautology is
// exactly what an earlier version of this test did.

func TestClone_BreaksBackingArraySharing(t *testing.T) {
	t.Parallel()
	p := Verified("usr_1", ToolDataEntry, "org", "slug", []string{"admin"})
	c := p.Clone()

	if &c.roles[0] == &p.roles[0] {
		t.Fatal("Clone shares the roles backing array with the original")
	}

	c.roles[0] = "superuser"
	if p.roles[0] != "admin" {
		t.Errorf("mutating the clone changed the original: %v", p.roles)
	}
}

func TestFrom_DoesNotAliasTheContextValue(t *testing.T) {
	t.Parallel()
	// The ctx value is shared by every reader for the life of a request, so
	// this is the widest exposure: without the clone, one consumer could
	// mutate a role another is about to authorize against.
	p := Verified("usr_1", ToolDataEntry, "org", "slug", []string{"admin"})
	ctx := With(context.Background(), p)

	a, b := From(ctx), From(ctx)

	if len(a.roles) == 0 || len(b.roles) == 0 {
		t.Fatal("precondition: From(ctx) returned no roles")
	}
	if &a.roles[0] == &b.roles[0] {
		t.Fatal("two From(ctx) results share the roles backing array")
	}

	// And neither aliases the value still held in the context.
	a.roles[0] = "superuser"
	if got := From(ctx); got.roles[0] != "admin" {
		t.Errorf("mutating a From() result corrupted the context value: %v", got.roles)
	}
}

func TestClone_ZeroRolesDoesNotAllocate(t *testing.T) {
	// No t.Parallel(): testing.AllocsPerRun is documented as invalid when the
	// test runs in parallel with others (it measures process-wide allocations).
	//
	// Clone is on the From(ctx) hot path, called on every principal read at
	// every entry point. The no-roles case — every CLI, MCP, scheduler and
	// header-auth request — must stay allocation-free, so only the JWT path
	// pays for the guarantee.
	p := Principal{User: "alice", Tool: ToolCLI}
	if n := testing.AllocsPerRun(100, func() { _ = p.Clone() }); n != 0 {
		t.Errorf("Clone allocated %v times for a role-less principal, want 0", n)
	}
}
