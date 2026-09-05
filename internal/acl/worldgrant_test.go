package acl_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestDefaultWorldNameMatchesMetamodel pins the duplicated constant.
// internal/acl may not import internal/metamodel (arch-lint), so the name
// is spelled twice; a drift would make a `world:default` grant stop being
// recognized as the default world.
func TestDefaultWorldNameMatchesMetamodel(t *testing.T) {
	t.Parallel()
	if acl.DefaultWorldName != metamodel.DefaultWorldName {
		t.Fatalf("acl.DefaultWorldName = %q, metamodel.DefaultWorldName = %q — "+
			"these must agree", acl.DefaultWorldName, metamodel.DefaultWorldName)
	}
}

// TestWorldGrantSplit_LeavesReadTypeOnly is the structural guarantee behind
// the whole representation choice: after load, Read holds nothing but
// entity types.
//
// If a world token survived in Read it would be intersected with a client
// ceiling's TYPE allow/deny list by filterTypes — silently DROPPED under an
// allowlist ceiling, silently KEPT under a deny ceiling — and reported by
// aclaudit's B1 as an undeclared entity type at High severity.
func TestWorldGrantSplit_LeavesReadTypeOnly(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"author": {Read: []string{"page", "world:editorial", "ticket", "world:published"}},
	}}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	role := p.Roles["author"]
	for _, entry := range role.Read {
		if strings.HasPrefix(entry, acl.WorldGrantPrefix) {
			t.Errorf("Read still holds a world token %q — it must be split into Worlds", entry)
		}
	}
	if got, want := strings.Join(role.Read, ","), "page,ticket"; got != want {
		t.Errorf("Read = %q, want %q", got, want)
	}
	if got, want := strings.Join(role.Worlds, ","), "editorial,published"; got != want {
		t.Errorf("Worlds = %q, want %q", got, want)
	}
}

// TestWorldGrantSplit_Idempotent pins the property NewDeclarative relies
// on: it runs the split too, and a policy that already came through
// Validate must not be mangled by the second pass.
func TestWorldGrantSplit_Idempotent(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"author": {Read: []string{"page", "world:published"}},
	}}
	if err := p.Validate(); err != nil {
		t.Fatalf("first Validate: %v", err)
	}
	first := p.Roles["author"]
	if err := p.Validate(); err != nil {
		t.Fatalf("second Validate: %v", err)
	}
	second := p.Roles["author"]
	readChanged := strings.Join(first.Read, ",") != strings.Join(second.Read, ",")
	worldsChanged := strings.Join(first.Worlds, ",") != strings.Join(second.Worlds, ",")
	if readChanged || worldsChanged {
		t.Errorf("not idempotent: %v/%v then %v/%v",
			first.Read, first.Worlds, second.Read, second.Worlds)
	}
}

func TestWorldGrantRejections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, grant, mustSay string
	}{
		{"empty world name", "world:", "names no world"},
		{"whitespace-only world name", "world:   ", "names no world"},
		{"world wildcard", "world:*", "name each world explicitly"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &acl.Policy{Roles: map[string]acl.RoleDef{
				"r": {Read: []string{tc.grant}},
			}}
			err := p.Validate()
			if err == nil {
				t.Fatalf("expected %q to be rejected at load", tc.grant)
			}
			if !strings.Contains(err.Error(), tc.mustSay) {
				t.Errorf("error must explain the problem (%q); got: %v", tc.mustSay, err)
			}
		})
	}
}

// TestStateGrant_WriteReadCoverage pins the F2 fix: the write⊆read
// invariant (TKT-4LQMWP) compares on the TYPE half.
//
// Before the fix, `update: ["policy@draft"]` was compared as a literal
// against the read list and failed load with a hint telling the operator to
// add "policy@draft" to `read:` — which a read list cannot hold.
func TestStateGrant_WriteReadCoverage(t *testing.T) {
	t.Parallel()
	t.Run("state grant covered by a bare-type read grant loads", func(t *testing.T) {
		t.Parallel()
		p := &acl.Policy{Roles: map[string]acl.RoleDef{
			"author": {Update: []string{"policy@draft"}, Read: []string{"policy"}},
		}}
		if err := p.Validate(); err != nil {
			t.Fatalf("update on policy@draft with read on policy must load: %v", err)
		}
	})
	t.Run("state grant with no read coverage is still rejected", func(t *testing.T) {
		t.Parallel()
		p := &acl.Policy{Roles: map[string]acl.RoleDef{
			"author": {Update: []string{"policy@draft"}, Read: []string{"page"}},
		}}
		err := p.Validate()
		if err == nil {
			t.Fatal("expected the write-without-read rejection")
		}
		if !strings.Contains(err.Error(), `add "policy"`) {
			t.Errorf("hint must name the TYPE, not the joined grant; got: %v", err)
		}
	})
}

func TestStateGrant_MalformedRejected(t *testing.T) {
	t.Parallel()
	for _, grant := range []string{
		"page@",      // no face
		"page@Draft", // uppercase: not the face grammar
		"page@a--b",  // consecutive hyphens
		"page@d@p",   // a state cannot have a state
		"@draft",     // no type
	} {
		t.Run(grant, func(t *testing.T) {
			t.Parallel()
			p := &acl.Policy{Roles: map[string]acl.RoleDef{
				"r": {Update: []string{grant}, Read: []string{"*"}},
			}}
			if err := p.Validate(); err == nil {
				t.Errorf("expected %q to be rejected at load", grant)
			}
		})
	}
}

// TestStateGrant_TypeNameWithSpaceParses pins why parseStateGrant does not
// reuse entity.ParseStateRef: that codec validates its left side with the
// ENTITY-ID grammar, which rejects the internal spaces
// metamodel.ValidateSchemaName deliberately permits in a type name.
func TestStateGrant_TypeNameWithSpaceParses(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"r": {Update: []string{"some property@draft"}, Read: []string{"some property"}},
	}}
	if err := p.Validate(); err != nil {
		t.Fatalf("a legal (if unusual) type name must be grantable: %v", err)
	}
}

// TestExistingPolicyUnchanged is the backward-compatibility guarantee: a
// policy with no world token means exactly what it meant before.
func TestExistingPolicyUnchanged(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{Roles: map[string]acl.RoleDef{
		"editor": {Read: []string{"page", "ticket"}, Update: []string{"page"}},
		"admin":  {Read: []string{"*"}, Create: []string{"*"}, Update: []string{"*"}, Delete: []string{"*"}},
	}}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for name, role := range p.Roles {
		if len(role.Worlds) != 0 {
			t.Errorf("role %q gained world grants from a policy that declared none: %v",
				name, role.Worlds)
		}
	}
	if got := strings.Join(p.Roles["editor"].Read, ","); got != "page,ticket" {
		t.Errorf("read list altered: %q", got)
	}
}

// TestGrantsVerbOnState pins the face-granular write check: exact match,
// no inheritance between states, and no wildcard over faces.
func TestGrantsVerbOnState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		grants []string
		target string
		face   string
		want   bool
	}{
		{"bare type grants the default state", []string{"page"}, "page", "", true},
		{"bare type does NOT grant a named state", []string{"page"}, "page", "draft", false},
		{"state grant grants its own face", []string{"page@draft"}, "page", "draft", true},
		{"state grant does NOT grant a sibling face", []string{"page@draft"}, "page", "published", false},
		{"state grant does NOT grant the default state", []string{"page@draft"}, "page", "", false},
		{"wildcard grants every type's default state", []string{"*"}, "page", "", true},
		{"wildcard does NOT grant a named state", []string{"*"}, "page", "draft", false},
		{"wrong type", []string{"page@draft"}, "policy", "draft", false},
		{"several grants, one matches", []string{"ticket", "page@draft"}, "page", "draft", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			role := acl.RoleDef{Update: tc.grants}
			if got := acl.GrantsVerbOnState(role, acl.OpUpdate, tc.target, entity.Face(tc.face)); got != tc.want {
				t.Errorf("GrantsVerbOnState(%v, %q@%q) = %v, want %v",
					tc.grants, tc.target, tc.face, got, tc.want)
			}
		})
	}
}

// permitsWorldFor builds a Request for a principal holding the everyone
// role plus any assigned one, and asks it about a world.
func permitsWorldFor(t *testing.T, p *acl.Policy, world string) bool {
	t.Helper()
	if err := p.Validate(); err != nil {
		t.Fatalf("policy must load: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	req, err := d.ForPrincipal(principal.Principal{User: "alice", Tool: "test"})
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	ok, err := req.PermitsWorld(context.Background(), world)
	if err != nil {
		t.Fatalf("PermitsWorld: %v", err)
	}
	return ok
}

// TestPermitsWorld is the composition test for the exported gate PR-C
// builds on: ceiling -> Globals -> roleFor -> roleGrantsWorldRead.
func TestPermitsWorld(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		roles map[string]acl.RoleDef
		world string
		want  bool
	}{
		{
			name:  "named world grant permits that world",
			roles: map[string]acl.RoleDef{"everyone": {Read: []string{"world:published"}}},
			world: "published", want: true,
		},
		{
			name:  "a world the principal was not granted is denied",
			roles: map[string]acl.RoleDef{"everyone": {Read: []string{"world:published"}}},
			world: "editorial", want: false,
		},
		{
			name:  "an ordinary read grant permits the default world",
			roles: map[string]acl.RoleDef{"everyone": {Read: []string{"page"}}},
			world: "", want: true,
		},
		{
			name:  "a bare read grant does NOT permit a non-default world",
			roles: map[string]acl.RoleDef{"everyone": {Read: []string{"page"}}},
			world: "published", want: false,
		},
		{
			name:  "the read wildcard does NOT reach a non-default world",
			roles: map[string]acl.RoleDef{"everyone": {Read: []string{"*"}}},
			world: "published", want: false,
		},
		{
			name:  "a principal holding no role at all is denied",
			roles: map[string]acl.RoleDef{},
			world: "published", want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := permitsWorldFor(t, &acl.Policy{Roles: tc.roles}, tc.world)
			if got != tc.want {
				t.Errorf("PermitsWorld(%q) = %v, want %v", tc.world, got, tc.want)
			}
		})
	}
}

// TestPermitsWorld_CeilingOverridesRoleGrant pins that the client ceiling
// is consulted — the roleFor clamp point plus the mandatory permitsWorld
// post-check. A restricted client must not reach a world its user holds.
func TestPermitsWorld_CeilingOverridesRoleGrant(t *testing.T) {
	t.Parallel()
	p := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"everyone": {Read: []string{"page", "world:published", "world:editorial"}},
		},
		ClientBaselines: map[string]acl.ClientBaseline{
			"app": {AppliesTo: []string{"app"}, Restriction: acl.Restriction{
				Worlds: []string{"published"},
			}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("policy must load: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	req, err := d.ForPrincipal(principal.VerifiedFrom("alice", "test",
		principal.Claims{PrincipalType: "app"}))
	if err != nil {
		t.Fatalf("ForPrincipal: %v", err)
	}
	ctx := context.Background()
	for _, tc := range []struct {
		world string
		want  bool
		why   string
	}{
		{"published", true, "inside the ceiling's allowlist"},
		{"editorial", false, "the user holds it but the ceiling does not"},
		{"", true, "an allowlist must not revoke the default world"},
	} {
		got, err := req.PermitsWorld(ctx, tc.world)
		if err != nil {
			t.Fatalf("PermitsWorld(%q): %v", tc.world, err)
		}
		if got != tc.want {
			t.Errorf("PermitsWorld(%q) = %v, want %v (%s)", tc.world, got, tc.want, tc.why)
		}
	}
}

// TestReadFaces pins the exported accessor a capability DISPLAY reads.
//
// GrantsRead alone overstates a face-scoped grant — it is true for both
// `read: [page]` and `read: [page@draft]`, because a face grant does grant its
// type for the purposes of composing a query. Which FACES is the separate
// question, and rendering a table without it shows a role limited to one face
// as identical to one reading every face.
func TestReadFaces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		read      []string
		wantAll   bool
		wantFaces []string
	}{
		{
			name:    "a bare type grant reads every face",
			read:    []string{"page"},
			wantAll: true,
		},
		{
			name:    "a wildcard reads every face",
			read:    []string{"*"},
			wantAll: true,
		},
		{
			name:      "a face grant is narrowed to that face",
			read:      []string{"page@draft"},
			wantFaces: []string{"draft"},
		},
		{
			name:      "two face grants accumulate",
			read:      []string{"page@draft", "page@published"},
			wantFaces: []string{"draft", "published"},
		},
		{
			// A bare grant beside a face grant WIDENS: the role reads every
			// face, so reporting the narrow set would understate access.
			name:    "a bare grant beside a face grant wins",
			read:    []string{"page@draft", "page"},
			wantAll: true,
		},
		{
			name: "another type's face grant does not leak",
			read: []string{"ticket@draft"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			faces, all := acl.ReadFaces(acl.RoleDef{Read: tc.read}, "page")
			if all != tc.wantAll {
				t.Fatalf("all = %v, want %v", all, tc.wantAll)
			}
			got := make([]string, 0, len(faces))
			for _, f := range faces {
				got = append(got, f.String())
			}
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(tc.wantFaces, ",") {
				t.Errorf("faces = %v, want %v", got, tc.wantFaces)
			}
		})
	}
}
