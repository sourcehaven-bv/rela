package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// aclModeGuard is the ACL-mode rule's failure vocabulary (TKT-0NODIZ). A
// decision that depends on which ACL is configured (no policy, read-only, a
// declarative policy) goes through access.ModeOf and a Mode method. A type
// switch or assertion on acl.NopACL or acl.ReadOnlyACL elsewhere rebuilds one
// of the switches access.Mode replaced, and such switches drifted apart: one
// matched only the value form, so `&acl.ReadOnlyACL{}` slipped through.
var aclModeGuard = guard{
	what: "ACL mode switch",
	list: "aclModeAllowlist",
	advice: "classify the ACL with access.ModeOf and ask a Mode method; add a method there if " +
		"none fits",
}

// The guard matches the import name acl only, and only type switches and
// type assertions. An aliased import or an equality check such as
// `a == acl.NopACL{}` passes; review catches those, as no such code exists.
//
// aclModeNames are the ACL implementations whose type names a mode check.
var aclModeNames = map[string]bool{"NopACL": true, "ReadOnlyACL": true}

// aclModeChecks returns the position of every acl.NopACL or acl.ReadOnlyACL
// type, value or pointer form, named in a type switch case or a type
// assertion.
func aclModeChecks(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	check := func(expr ast.Expr) {
		if star, ok := expr.(*ast.StarExpr); ok {
			expr = star.X
		}
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok || !aclModeNames[sel.Sel.Name] {
			return
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "acl" {
			found = append(found, fset.Position(sel.Sel.Pos()))
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.TypeSwitchStmt:
			for _, stmt := range n.Body.List {
				if clause, ok := stmt.(*ast.CaseClause); ok {
					for _, typ := range clause.List {
						check(typ)
					}
				}
			}
		case *ast.TypeAssertExpr:
			if n.Type != nil {
				check(n.Type)
			}
		}
		return true
	})
	return found
}

func TestNoACLModeSwitches(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, aclModeChecks)
	checkAllowlist(t, aclModeGuard, got, aclModeAllowlist)
}

func TestACLModeAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, aclModeAllowlist)
}

func TestACLModeChecks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"value and pointer case", `switch a.(type) { case acl.ReadOnlyACL, *acl.ReadOnlyACL: }`, 2},
		{"nop case", `switch x := a.(type) { case acl.NopACL: _ = x }`, 1},
		{"assertion", `_, ok := a.(acl.NopACL); _ = ok`, 1},
		{"pointer assertion", `_, ok := a.(*acl.ReadOnlyACL); _ = ok`, 1},
		{"declarative is not a mode name", `_, ok := a.(*acl.Declarative); _ = ok`, 0},
		{"other package", `switch a.(type) { case other.NopACL: }`, 0},
		{"composite literal", `_ = acl.ReadOnlyACL{}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "package p\nfunc f() {\n" + tc.body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := aclModeChecks(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
