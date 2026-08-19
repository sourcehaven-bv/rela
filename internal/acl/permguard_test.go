package acl

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// exemptPermConstants are `Perm*` string constants in this package that are
// deliberately NOT global named permissions, and so must not appear in
// [BuiltinPermissions].
//
// The list is an EXEMPTION list, not an inclusion list, so a newly added
// constant fails closed: it must either be registered in BuiltinPermissions or
// be explicitly exempted here with a reason. An inclusion list would silently
// skip a new constant — the same forget-shaped hole that produced the bug this
// guard exists to prevent.
var exemptPermConstants = map[string]string{}

// permConstDecl matches a package-level `Perm<Name> = "<value>"` string
// constant, in both the single-line `const X = "y"` form and the grouped
// `const ( X = "y" )` form.
var permConstDecl = regexp.MustCompile(`^(?:const\s+)?(Perm\w+)\s+=\s+"([^"]+)"`)

// TestBuiltinPermissions_CoversEveryPermConstant is the structural guarantee
// behind the A7 dead-permission check.
//
// `rela acl audit` reports a granted permission as dead config when nothing
// references it, and advises removing it. rela's own global permissions are
// granted through a role's `permissions:` list but consumed by read paths, so
// [BuiltinPermissions] is what tells the audit they are live. A new global
// permission constant that never lands in that list is therefore reported as
// dead the first time an operator grants it, with a hint to delete a working
// grant.
//
// That failure is invisible: the audit is advisory, so nothing breaks in CI —
// the damage lands on whoever trusts the remediation. A test that merely
// iterates BuiltinPermissions cannot catch it, because it only ever sees what
// is already registered. This scans the source instead.
func TestBuiltinPermissions_CoversEveryPermConstant(t *testing.T) {
	t.Parallel()
	registered := BuiltinPermissions()

	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	found := 0
	for _, name := range all {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			m := permConstDecl.FindStringSubmatch(strings.TrimSpace(line))
			if m == nil {
				continue
			}
			constName, value := m[1], m[2]
			if _, exempt := exemptPermConstants[constName]; exempt {
				continue
			}
			found++
			if !slices.Contains(registered, value) {
				t.Errorf("%s:%d: %s = %q is not returned by BuiltinPermissions(), so "+
					"`rela acl audit` will report it as dead config and advise removing "+
					"a working grant — add it to BuiltinPermissions, or exempt it in "+
					"exemptPermConstants with a reason",
					name, i+1, constName, value)
			}
		}
	}

	// A guard that scans nothing passes silently. Assert it found real work, so
	// a broken glob, a renamed constant convention, or an over-broad exemption
	// list surfaces as a failure rather than a green run.
	if found == 0 {
		t.Fatal("scanned no Perm* constants — the guard is not doing its job " +
			"(check permConstDecl against the current declaration style)")
	}

	// Every registered value must correspond to a real constant, so a permission
	// deleted from the package cannot linger as a permanent A7 exemption.
	if len(registered) != found {
		t.Errorf("BuiltinPermissions() returns %d permissions but %d Perm* constants were "+
			"found in source; a stale entry would permanently exempt a permission that no "+
			"longer exists", len(registered), found)
	}
}
