package acl

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// exemptFiles are the files in this package NOT scanned for direct role
// lookups. Everything else is scanned.
//
// The list is an EXEMPTION list, not an inclusion list, so a newly added file
// fails closed: it must either be clean or be explicitly exempted here with a
// reason. An inclusion list would silently skip a new evaluation path — the
// same forget-shaped hole `applies_to` disjointness exists to remove.
var exemptFiles = map[string]string{
	// The policy's own accessors and load-time compilation legitimately read
	// role definitions: they answer "what does this policy declare?", which has
	// no principal and therefore no ceiling.
	"policy.go":         "policy structure, no principal in scope",
	"ceiling.go":        "ceiling definition + load-time validation",
	"ceilingcompile.go": "the clamp itself — reads roles to narrow them",
	"declarative.go":    "constructor and policy accessors",
	"request.go":        "defines roleFor, the clamp point",
}

// directRoleLookup matches `<something>.Roles[` — the pattern that reaches a
// RoleDef without passing through the ceiling.
var directRoleLookup = regexp.MustCompile(`policy\.Roles\[`)

// declarationCheck matches the two shapes that only ask whether a role NAME is
// declared, discarding the RoleDef. Those cannot leak capability: they gate
// whether an attribution is added at all, and the resulting role is clamped
// later when a predicate is asked of it.
var declarationCheck = regexp.MustCompile(`_,\s*(ok|defined|hasEveryone)\s*:?=`)

// policyWideReporters are functions with NO principal in scope, so there is no
// ceiling to apply. They answer questions about the policy itself ("does the
// everyone role grant read on ticket?", "which claims map to a granting
// role?"), not about one principal's effective access — a ceiling would be
// meaningless, since it is a property of a caller and these have none.
//
// Keep this list tiny. A function that HAS a principal and reports its access
// belongs on Request and must clamp (grantingAttributions does).
var policyWideReporters = map[string]bool{
	"EveryoneGrants": true,
	"AssertedGrants": true,
}

// enclosingFunc scans backwards for the nearest `func` declaration, so a
// finding can be attributed to the function containing it.
func enclosingFunc(lines []string, idx int) string {
	re := regexp.MustCompile(`^func(?:\s+\([^)]*\))?\s+(\w+)`)
	for i := idx; i >= 0; i-- {
		if m := re.FindStringSubmatch(lines[i]); m != nil {
			return m[1]
		}
	}
	return ""
}

// TestNoDirectRoleLookupInEvaluationPaths is the structural guarantee behind
// client attenuation.
//
// The ceiling is enforced by narrowing the RoleDef in [Request.roleFor], which
// works precisely BECAUSE every evaluation path resolves role names through it.
// A future path that reaches into policy.Roles directly would silently evaluate
// against un-attenuated grants — a client restriction that reads correctly in
// acl.yaml and does nothing at runtime.
//
// That failure is invisible: no error, no test breakage, just a restricted
// client quietly holding its user's full authority. Prose in roleFor's doc
// cannot catch it; this can.
func TestNoDirectRoleLookupInEvaluationPaths(t *testing.T) {
	t.Parallel()
	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	scanned := 0
	for _, name := range all {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, exempt := exemptFiles[name]; exempt {
			continue
		}
		scanned++
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(filepath.Clean(name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			lines := strings.Split(string(src), "\n")
			for i, line := range lines {
				if !directRoleLookup.MatchString(line) {
					continue
				}
				if declarationCheck.MatchString(line) {
					continue // existence check, not a capability read
				}
				if fn := enclosingFunc(lines, i); policyWideReporters[fn] {
					continue // no principal in scope; see policyWideReporters
				}
				t.Errorf("%s:%d: direct policy.Roles lookup bypasses the client "+
					"ceiling — use Request.roleFor instead:\n\t%s",
					name, i+1, strings.TrimSpace(line))
			}
		})
	}

	// A guard that scans nothing passes silently. Assert it found real work,
	// so a broken glob or an over-broad exemption list surfaces as a failure
	// rather than a green run.
	if scanned < 5 {
		t.Errorf("scanned only %d files; the guard is not covering the package", scanned)
	}
}

// TestRoleFor_AppliesTheCeiling is the positive half: roleFor must actually
// narrow, or the guard above protects nothing.
func TestRoleFor_AppliesTheCeiling(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
roles:
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    permissions: [history:read]
client_baselines:
  apps:
    applies_to: [app]
    read: [ticket]
    deny_write: ["*"]
    deny_permissions: [history:read]
`)
	d, err := NewDeclarative(p, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}

	t.Run("attenuated client", func(t *testing.T) {
		t.Parallel()
		req, err := d.ForPrincipal(verifiedClient("svc", "app"))
		if err != nil {
			t.Fatalf("ForPrincipal: %v", err)
		}
		role, ok := req.roleFor("admin")
		if !ok {
			t.Fatal("admin role not found")
		}
		if !equalStrings(role.Read, []string{"ticket"}) {
			t.Errorf("Read = %v, want [ticket] — the wildcard must collapse to the ceiling", role.Read)
		}
		// deny_write: ["*"] is a PURE denial, so filterTypes preserves the
		// role's wildcard (a plain list cannot spell "all except ...") and the
		// denial is enforced at match time instead. Assert the predicate, which
		// is what actually gates — not the list, which is expected to look
		// permissive here.
		for _, op := range []Op{OpCreate, OpUpdate, OpDelete} {
			if req.ceiling.permitsVerb(op, "ticket") {
				t.Errorf("deny_write did not deny %s", op)
			}
		}
		if req.ceiling.permitsPermission("history:read") {
			t.Error("deny_permissions did not withhold history:read")
		}
		if !equalStrings(role.Permissions, []string{}) && len(role.Permissions) > 0 {
			t.Errorf("Permissions = %v, want the withheld permission filtered out", role.Permissions)
		}
	})

	t.Run("interactive user is untouched", func(t *testing.T) {
		t.Parallel()
		req, err := d.ForPrincipal(verifiedClient("alice", "user"))
		if err != nil {
			t.Fatalf("ForPrincipal: %v", err)
		}
		role, ok := req.roleFor("admin")
		if !ok {
			t.Fatal("admin role not found")
		}
		for name, got := range map[string][]string{
			"Read": role.Read, "Create": role.Create,
			"Update": role.Update, "Delete": role.Delete,
		} {
			if !equalStrings(got, []string{"*"}) {
				t.Errorf("%s = %v, want [*] — an interactive user must be unattenuated", name, got)
			}
		}
		if !equalStrings(role.Permissions, []string{"history:read"}) {
			t.Errorf("Permissions = %v, want [history:read]", role.Permissions)
		}
	})
}

// verifiedClient builds a principal as the JWT gate would: claims populated
// through the Verified constructor, which is the only way they can be set.
func verifiedClient(user, principalType string) principal.Principal {
	return principal.VerifiedFrom(user, principal.ToolDataEntry, principal.Claims{
		PrincipalType: principalType,
	})
}

// TestCeiling_BothPermissionEntryPointsClamp pins that the ENTITY-SCOPED
// permission check is attenuated too, not just the global one.
//
// HoldsPermissionForEntity is what the state-machine transition guard uses
// (via affordances.requestGuard), so a gap here would let a restricted client
// perform a guarded transition the ceiling withholds. Both entry points funnel
// through grantsPermission today; this fails if a future refactor gives the
// subject-aware path its own lookup.
func TestCeiling_BothPermissionEntryPointsClamp(t *testing.T) {
	t.Parallel()
	p := mustPolicy(t, `
roles:
  approver:
    read: ["*"]
    permissions: [establish]
assignments:
  alice: approver
client_baselines:
  apps:
    applies_to: [app]
    deny_permissions: [establish]
`)
	d, err := NewDeclarative(p, NullGraph{}, NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	ctx := context.Background()

	unrestricted, err := d.ForPrincipal(verifiedClient("alice", "user"))
	if err != nil {
		t.Fatal(err)
	}
	if !unrestricted.HoldsPermission(ctx, "establish") {
		t.Fatal("precondition: alice should hold establish")
	}
	if !unrestricted.HoldsPermissionForEntity(ctx, "TKT-1", "establish") {
		t.Fatal("precondition: alice should hold establish for the entity")
	}

	client, err := d.ForPrincipal(verifiedClient("alice", "app"))
	if err != nil {
		t.Fatal(err)
	}
	if client.HoldsPermission(ctx, "establish") {
		t.Error("global HoldsPermission ignored deny_permissions")
	}
	if client.HoldsPermissionForEntity(ctx, "TKT-1", "establish") {
		t.Error("entity-scoped HoldsPermissionForEntity ignored deny_permissions — " +
			"a restricted client could perform a guarded transition")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
