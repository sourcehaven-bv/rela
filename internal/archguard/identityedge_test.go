package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// aclTree is the acl package and its helpers. The identity guards here pin
// callers OUTSIDE it; inside, TestEvaluatorDoesNotResolveIdentity keeps the
// evaluator from resolving.
const aclTree = "internal/acl"

// identityEdgeGuard is the identity-edge rule's failure vocabulary.
var identityEdgeGuard = guard{
	what: "identity resolution call",
	list: "identityEdgeAllowlist",
	advice: "resolve the principal once at the request edge and read the attached Request below it " +
		"(acl.Declarative.RequestFrom). A transport whose principal is already a graph identity " +
		"takes appbuild.IdentityBinder at wiring instead of calling the resolver itself. " +
		"If this file IS a new request edge, add it to identityEdgeAllowlist with the reason",
}

// fromContextGuard is the FromContext rule's failure vocabulary. Its
// allowlist is empty: every caller outside internal/acl must use RequestFrom.
var fromContextGuard = guard{
	what: "acl.FromContext call",
	list: "fromContextAllowlist",
	advice: "use acl.Declarative.RequestFrom, which also checks that the attached Request belongs " +
		"to this Declarative and to the principal on ctx, and fails closed otherwise",
}

// fromContextAllowlist pins acl.FromContext callers outside internal/acl. It
// is empty and may only stay empty.
var fromContextAllowlist = map[string]allowed{}

// identityEdgeMethods are the acl methods that resolve an identity or open a
// Request for one. No other package declares a method with these names, so
// they are matched on any receiver.
var identityEdgeMethods = map[string]bool{
	"ForIdentity":      true, // Declarative.ForIdentity: opens a Request for a resolved Identity
	"ResolveDirect":    true, // IdentityResolver.ResolveDirect
	"ResolvePrincipal": true, // IdentityResolver.ResolvePrincipal
}

// identityEdgeCalls returns the position of every reference that resolves an
// identity, called or taken as a value:
//
//   - `x.IdentityResolver`, the accessor or the acl type, except a call with
//     arguments (another package's method of that name). Every
//     IdentityResolver.Resolve call needs a resolver, and this is the only
//     way to name one, so Resolve itself is not matched: the name is too
//     common to match on any receiver.
//   - `x.ForIdentity`, `x.ResolveDirect`, `x.ResolvePrincipal`.
//   - `acl.OpenRequest`, `acl.BindRequest`, under any import name.
//   - `identityBinderFor`, appbuild's binder constructor.
//
// Syntactic, like the other guards here.
func identityEdgeCalls(fset *token.FileSet, file *ast.File) []token.Position {
	aclName := importName(file, "github.com/Sourcehaven-BV/rela/internal/acl")
	calledWithArgs := map[*ast.SelectorExpr]bool{}
	declared := map[*ast.Ident]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && len(n.Args) > 0 {
				calledWithArgs[sel] = true
			}
		case *ast.FuncDecl:
			declared[n.Name] = true
		case *ast.SelectorExpr:
			declared[n.Sel] = true // matched as a selector below, not as a bare name
		}
		return true
	})
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			name := n.Sel.Name
			switch {
			case name == "IdentityResolver" && !calledWithArgs[n],
				identityEdgeMethods[name],
				name == "identityBinderFor",
				(name == "OpenRequest" || name == "BindRequest") && isPkg(n.X, aclName):
				found = append(found, fset.Position(n.Sel.Pos()))
			}
		case *ast.Ident:
			if n.Name == "identityBinderFor" && !declared[n] {
				found = append(found, fset.Position(n.Pos()))
			}
		}
		return true
	})
	return found
}

// fromContextCalls returns the position of every `acl.FromContext` call,
// under any import name.
func fromContextCalls(fset *token.FileSet, file *ast.File) []token.Position {
	aclName := importName(file, "github.com/Sourcehaven-BV/rela/internal/acl")
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "FromContext" && isPkg(sel.X, aclName) {
			found = append(found, fset.Position(sel.Sel.Pos()))
		}
		return true
	})
	return found
}

// importName returns the name file uses for the package at path: its alias,
// or the last path element. It returns "" when file does not import it.
func importName(file *ast.File, path string) string {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) != path {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return path[strings.LastIndex(path, "/")+1:]
	}
	return ""
}

func isPkg(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && name != "" && id.Name == name
}

// outsideACL drops the findings under internal/acl.
func outsideACL(got map[string][]token.Position) map[string][]token.Position {
	for path := range got {
		if path == aclTree || strings.HasPrefix(path, aclTree+"/") {
			delete(got, path)
		}
	}
	return got
}

// TestIdentityEdgeCallers pins every call that resolves an identity outside
// internal/acl to identityEdgeAllowlist, exactly. Identity is resolved once,
// at the request edge; a resolve call anywhere else is an evaluation path
// resolving for itself, which is the per-evaluation walk the edge removed.
func TestIdentityEdgeCallers(t *testing.T) {
	t.Parallel()
	got := outsideACL(scanTree(t, repoRoot, scannedRoots, identityEdgeCalls))
	if len(got) == 0 {
		t.Fatal("scanned no identity edges at all; the walk is probably rooted wrong")
	}
	checkAllowlist(t, identityEdgeGuard, got, identityEdgeAllowlist)
}

func TestIdentityEdgeAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, identityEdgeAllowlist)
}

// TestNoACLFromContext forbids acl.FromContext outside internal/acl. It does
// not check which Declarative or principal the Request belongs to, so a gate
// built on it can evaluate the wrong identity.
func TestNoACLFromContext(t *testing.T) {
	t.Parallel()
	got := outsideACL(scanTree(t, repoRoot, scannedRoots, fromContextCalls))
	checkAllowlist(t, fromContextGuard, got, fromContextAllowlist)
}

func TestIdentityMatchers(t *testing.T) {
	t.Parallel()
	const imp = "import acl \"github.com/Sourcehaven-BV/rela/internal/acl\"\n"
	const aliased = "import a \"github.com/Sourcehaven-BV/rela/internal/acl\"\n"
	cases := []struct {
		name    string
		imports string
		body    string
		match   matcher
		want    int
	}{
		{"resolver accessor", imp, `_ = d.IdentityResolver()`, identityEdgeCalls, 1},
		{"resolver with args", imp, `_ = x.IdentityResolver(1)`, identityEdgeCalls, 0},
		{"resolver method value", imp, `f := d.IdentityResolver; _ = f`, identityEdgeCalls, 1},
		{"resolver type", imp, `var r *acl.IdentityResolver; _ = r`, identityEdgeCalls, 1},
		{"for identity", imp, `_, _ = d.ForIdentity(id)`, identityEdgeCalls, 1},
		{"resolve direct", imp, `_, _ = r.ResolveDirect(ctx, p)`, identityEdgeCalls, 1},
		{"resolve principal", imp, `_, _ = r.ResolvePrincipal(ctx, raw)`, identityEdgeCalls, 1},
		{"resolve principal value", imp, `f := r.ResolvePrincipal; _ = f`, identityEdgeCalls, 1},
		{"binder constructor", imp, `_ = identityBinderFor(d)`, identityEdgeCalls, 1},
		{"binder constructor value", imp, `f := identityBinderFor; _ = f`, identityEdgeCalls, 1},
		{"bind request value", imp, `f := acl.BindRequest; _ = f`, identityEdgeCalls, 1},
		{"other Resolve", imp, `_, _ = r.Resolve(ctx, p)`, identityEdgeCalls, 0},
		{"open request", imp, `_, _ = acl.OpenRequest(ctx, d, p)`, identityEdgeCalls, 1},
		{"bind request", imp, `_, _ = acl.BindRequest(ctx, a)`, identityEdgeCalls, 1},
		{"aliased bind", aliased, `_, _ = a.BindRequest(ctx, x)`, identityEdgeCalls, 1},
		{"other package's BindRequest", imp, `_, _ = r.BindRequest(ctx)`, identityEdgeCalls, 0},
		{"no acl import", "", `_, _ = acl.BindRequest(ctx, a)`, identityEdgeCalls, 0},
		{"from context", imp, `_ = acl.FromContext(ctx)`, fromContextCalls, 1},
		{"aliased from context", aliased, `_ = a.FromContext(ctx)`, fromContextCalls, 1},
		{"another FromContext", imp, `_ = audit.FromContext(ctx)`, fromContextCalls, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "package p\n" + tc.imports + "func f() {\n" + tc.body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := tc.match(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}

// Both guards must fail on a planted violation end to end, skip tests, and
// leave internal/acl alone.
func TestIdentityGuards_FindPlantedViolations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const src = "import \"github.com/Sourcehaven-BV/rela/internal/acl\"\n" +
		"func f() { _ = acl.FromContext(ctx); _, _ = acl.BindRequest(ctx, a) }\n"
	for rel, pkg := range map[string]string{
		"internal/pkg/bad.go":        "pkg",
		"internal/pkg/bad_test.go":   "pkg",
		"internal/acl/inside.go":     "acl",
		"internal/acl/acltest/h.go":  "acltest",
		"cmd/tool/main.go":           "main",
		"internal/pkg/testdata/x.go": "x",
	} {
		if err := mkdirAllWrite(filepath.Join(root, filepath.FromSlash(rel)), "package "+pkg+"\n"+src); err != nil {
			t.Fatal(err)
		}
	}
	for name, match := range map[string]matcher{"edge": identityEdgeCalls, "fromcontext": fromContextCalls} {
		got := outsideACL(scanTree(t, root, scannedRoots, match))
		if len(got) != 2 || len(got["internal/pkg/bad.go"]) != 1 || len(got["cmd/tool/main.go"]) != 1 {
			t.Errorf("%s: scan = %v, want one finding in each of internal/pkg/bad.go and cmd/tool/main.go", name, got)
		}
	}
	msgs := diffAllowlist(fromContextGuard, outsideACL(scanTree(t, root, scannedRoots, fromContextCalls)),
		counts(fromContextAllowlist))
	if len(msgs) != 2 {
		t.Errorf("fromContext: want two findings against the empty allowlist; got %q", msgs)
	}
}

// The edge guard must fail on a file that reaches the resolver without
// calling the accessor: a method value, a resolver taken by type, and a
// direct ForIdentity, in a file that is not on the allowlist.
func TestIdentityEdgeGuard_FindsPlantedIndirectViolation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const src = "package pkg\n" +
		"import \"github.com/Sourcehaven-BV/rela/internal/acl\"\n" +
		"func f(d *acl.Declarative, r *acl.IdentityResolver) {\n" +
		"\tresolve := r.ResolvePrincipal\n\t_ = resolve\n" +
		"\t_, _ = d.ForIdentity(acl.Identity{})\n}\n"
	if err := mkdirAllWrite(filepath.Join(root, "internal", "pkg", "sneaky.go"), src); err != nil {
		t.Fatal(err)
	}
	if err := mkdirAllWrite(filepath.Join(root, "cmd", "tool", "main.go"), "package main\n"); err != nil {
		t.Fatal(err)
	}
	got := outsideACL(scanTree(t, root, scannedRoots, identityEdgeCalls))
	if n := len(got["internal/pkg/sneaky.go"]); n != 3 {
		t.Errorf("findings in sneaky.go = %d (%v), want 3: the resolver type, the method value and ForIdentity",
			n, got)
	}
	if msgs := diffAllowlist(identityEdgeGuard, got, counts(identityEdgeAllowlist)); len(msgs) == 0 {
		t.Error("the planted file passed the allowlist check")
	}
}
