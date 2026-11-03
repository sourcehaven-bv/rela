package acl

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestResolve_SubstitutesAndKeepsClaims: a matched natural key replaces User
// with the entity id, keeps the raw value in RawUser, and keeps every
// verified claim. A dropped claim would remove a client ceiling.
func TestResolve_SubstitutesAndKeepsClaims(t *testing.T) {
	ctx := context.Background()
	d := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML,
		[]personRow{{id: "PERS-JV", email: "jv@example.com"}},
		[][3]string{{"PERS-JV", "heeft_rol", "ROLE-MD"}})

	raw := principal.VerifiedFrom("jv@example.com", principal.ToolDataEntry, principal.Claims{
		OrgID: "org-1", Roles: []string{"editor"}, PrincipalType: "agent", Scopes: []string{"read"},
	})
	id, err := d.IdentityResolver().Resolve(ctx, raw)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got := id.Principal()
	if got.User != "PERS-JV" || got.RawUser != "jv@example.com" {
		t.Errorf("User=%q RawUser=%q, want PERS-JV / jv@example.com", got.User, got.RawUser)
	}
	if got.OrgID() != "org-1" || got.PrincipalType() != "agent" ||
		!slices.Equal(got.Roles(), []string{"editor"}) || !slices.Equal(got.Scopes(), []string{"read"}) {

		t.Errorf("claims lost in substitution: %+v", got)
	}
	if id.Unmatched() {
		t.Error("a matched key reported Unmatched")
	}
	if !slices.Contains(id.members, "ROLE-MD") {
		t.Errorf("members = %v, want the heeft_rol closure to include ROLE-MD", id.members)
	}
}

// TestResolve_UnmatchedKeepsPrincipal: an enabled lookup that finds nothing
// leaves the principal as it was and flags the identity.
func TestResolve_UnmatchedKeepsPrincipal(t *testing.T) {
	ctx := context.Background()
	d := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML, nil, nil)
	id, err := d.IdentityResolver().Resolve(ctx,
		principal.Principal{User: "nobody@example.com", Tool: principal.ToolDataEntry})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !id.Unmatched() || id.Principal().User != "nobody@example.com" {
		t.Errorf("Unmatched=%v User=%q, want true / nobody@example.com", id.Unmatched(), id.Principal().User)
	}
	// ResolveDirect never runs the lookup, so it never flags.
	direct, err := d.IdentityResolver().ResolveDirect(ctx,
		principal.Principal{User: "nobody@example.com", Tool: principal.ToolDataEntry})
	if err != nil || direct.Unmatched() {
		t.Errorf("ResolveDirect: Unmatched=%v err=%v, want false / nil", direct.Unmatched(), err)
	}
}

// TestResolve_LookupFailureIsAnError: an ambiguous natural key is wrapped in
// ErrPrincipalLookup rather than resolved to either entity.
func TestResolve_LookupFailureIsAnError(t *testing.T) {
	ctx := context.Background()
	d := buildPrincipalLookupWorld(ctx, t, principalLookupPolicyYAML,
		[]personRow{{id: "PERS-A", email: "dup@example.com"}, {id: "PERS-B", email: "dup@example.com"}}, nil)
	_, err := d.IdentityResolver().Resolve(ctx,
		principal.Principal{User: "dup@example.com", Tool: principal.ToolDataEntry})
	if !errors.Is(err, ErrPrincipalLookup) {
		t.Fatalf("err = %v, want ErrPrincipalLookup", err)
	}
}

// failingGraph fails every membership read.
type failingGraph struct{ NullGraph }

func (failingGraph) OutgoingRelations(context.Context, string, string) ([]string, error) {
	return nil, errors.New("graph down")
}

// TestResolve_WalkErrorIsAnError: a failed member-of read aborts resolution.
// A partial closure would silently drop group grants.
func TestResolve_WalkErrorIsAnError(t *testing.T) {
	d, err := NewDeclarative(&Policy{Roles: map[string]RoleDef{"r": {Read: []string{"*"}}}},
		failingGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatal(err)
	}
	p := principal.Principal{User: "alice", Tool: principal.ToolCLI}
	if _, err := d.IdentityResolver().ResolveDirect(context.Background(), p); err == nil {
		t.Error("ResolveDirect: want the walk error, got nil")
	}
	if _, err := BindRequest(principal.With(context.Background(), p), d); err == nil {
		t.Error("BindRequest: want the walk error, got nil")
	}
}

// TestResolve_RejectsUnstamped: an unstamped principal has no identity.
func TestResolve_RejectsUnstamped(t *testing.T) {
	d, err := NewDeclarative(&Policy{}, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []principal.Principal{{}, {User: "unknown", Tool: principal.ToolCLI}, {User: "alice"}} {
		if _, err := d.IdentityResolver().ResolveDirect(context.Background(), p); !errors.Is(err, ErrUnstampedPrincipal) {
			t.Errorf("ResolveDirect(%+v) err = %v, want ErrUnstampedPrincipal", p, err)
		}
	}
}

// TestForIdentity_RejectsZeroAndForeign: ForIdentity only accepts an
// identity resolved by the same Declarative.
func TestForIdentity_RejectsZeroAndForeign(t *testing.T) {
	newD := func() *Declarative {
		d, err := NewDeclarative(&Policy{}, NullGraph{}, NullGraphQueryer{})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	d, other := newD(), newD()
	if _, err := d.ForIdentity(Identity{}); !errors.Is(err, ErrNoIdentity) {
		t.Errorf("zero Identity: err = %v, want ErrNoIdentity", err)
	}
	foreign, err := other.IdentityResolver().ResolveDirect(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolCLI})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.ForIdentity(foreign); !errors.Is(err, ErrForeignIdentity) {
		t.Errorf("foreign Identity: err = %v, want ErrForeignIdentity", err)
	}
}

// TestRequestFrom_FailsClosed: an evaluation path with no bound Request, or
// one bound by another Declarative, gets an error and no Request.
func TestRequestFrom_FailsClosed(t *testing.T) {
	d, err := NewDeclarative(&Policy{}, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewDeclarative(&Policy{}, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatal(err)
	}
	stamped := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	if _, gotErr := d.RequestFrom(stamped); !errors.Is(gotErr, ErrNoIdentity) {
		t.Errorf("stamped, unbound: err = %v, want ErrNoIdentity", gotErr)
	}
	bound, err := BindRequest(stamped, other)
	if err != nil {
		t.Fatal(err)
	}
	if _, gotErr := d.RequestFrom(bound); !errors.Is(gotErr, ErrForeignIdentity) {
		t.Errorf("bound by another Declarative: err = %v, want ErrForeignIdentity", gotErr)
	}
}
