package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// recreateGuard is the rule's failure vocabulary.
var recreateGuard = guard{
	what: "recreate entry point",
	list: "recreateAllowlist",
	advice: "entitymanager.RecreateEntity is history restore only: it runs no automation and does not " +
		"apply the state machines' entry rule (BUG-KK1UXH), so an ordinary create must use " +
		"Manager.CreateEntity. If you only moved an existing restore call between files, move its " +
		"allowlist entry with it",
}

// entitymanagerPath is the import path suffix of package entitymanager.
const entitymanagerPath = "/internal/entitymanager"

// recreateEntryPoints returns the position of every reference that reaches a
// create exempt from the entry rule:
//
//   - entitymanager.RecreateEntity or entitymanager.Recreator, under any
//     import name, as a call, a composite literal or a function value;
//   - a bare RecreateEntity or Recreator inside package entitymanager, or in
//     a file that dot-imports it (declaration names excepted);
//   - any `x.RecreateEntity` selector, which is how a consumer calls the
//     Recreator through its own narrow interface.
//
// The last rule pins the call sites as well as the wiring, so a new consumer
// of an existing wiring still needs an allowlist entry.
func recreateEntryPoints(fset *token.FileSet, file *ast.File) []token.Position {
	names := map[string]bool{}
	bare := file.Name.Name == "entitymanager"
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !strings.HasSuffix(path, entitymanagerPath) {
			continue
		}
		switch {
		case imp.Name == nil:
			names["entitymanager"] = true
		case imp.Name.Name == ".":
			bare = true
		default:
			names[imp.Name.Name] = true
		}
	}

	// Declaration names are not references: `func RecreateEntity`, `type
	// Recreator`, a method's receiver type, an interface method or a struct
	// field.
	decl := map[*ast.Ident]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			decl[n.Name] = true
			if n.Recv != nil && len(n.Recv.List) == 1 {
				recv := n.Recv.List[0].Type
				if star, ok := recv.(*ast.StarExpr); ok {
					recv = star.X
				}
				if id, ok := recv.(*ast.Ident); ok {
					decl[id] = true
				}
			}
		case *ast.TypeSpec:
			decl[n.Name] = true
		case *ast.Field:
			for _, id := range n.Names {
				decl[id] = true
			}
		}
		return true
	})

	var found []token.Position
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			pkg, isIdent := n.X.(*ast.Ident)
			switch {
			case n.Sel.Name == "RecreateEntity":
				found = append(found, fset.Position(n.Sel.Pos()))
			case n.Sel.Name == "Recreator" && isIdent && names[pkg.Name]:
				found = append(found, fset.Position(n.Sel.Pos()))
			}
			// Sel is a field or method name, never a bare reference.
			ast.Inspect(n.X, visit)
			return false
		case *ast.Ident:
			if bare && !decl[n] && (n.Name == "RecreateEntity" || n.Name == "Recreator") {
				found = append(found, fset.Position(n.Pos()))
			}
		}
		return true
	}
	ast.Inspect(file, visit)
	return found
}

// TestRecreateOnlyFromRestore pins every entry point to RecreateEntity to
// recreateAllowlist, so the entry-rule exemption stays unreachable from an
// ordinary create over HTTP, MCP, Lua or the CLI.
func TestRecreateOnlyFromRestore(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, recreateEntryPoints)
	if len(got) == 0 {
		t.Fatal("scanned no recreate entry points at all; the walk is probably rooted wrong")
	}
	checkAllowlist(t, recreateGuard, got, recreateAllowlist)
}

func TestRecreateAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, recreateAllowlist)
}

func TestRecreateEntryPoints(t *testing.T) {
	t.Parallel()
	const imp = `import "github.com/Sourcehaven-BV/rela/internal/entitymanager"`
	const aliased = `import em "github.com/Sourcehaven-BV/rela/internal/entitymanager"`
	const dotted = `import . "github.com/Sourcehaven-BV/rela/internal/entitymanager"`
	cases := []struct {
		name, pkg, imports, body string
		want                     int
	}{
		{"package function", "p", imp, `entitymanager.RecreateEntity(ctx, m, e)`, 1},
		{"adapter literal", "p", imp, `r := entitymanager.Recreator{M: m}; _ = r`, 1},
		{"function value", "p", imp, `f := entitymanager.RecreateEntity; _ = f`, 1},
		{"aliased import", "p", aliased, `em.RecreateEntity(ctx, m, e); _ = em.Recreator{M: m}`, 2},
		{"dot import", "p", dotted, `RecreateEntity(ctx, m, e); _ = Recreator{M: m}`, 2},
		{"bare call in package", "entitymanager", "", `RecreateEntity(ctx, m, e)`, 1},
		{"bare value in package", "entitymanager", "", `f := RecreateEntity; _ = f`, 1},
		{"consumer interface call", "p", "", `a.recreator.RecreateEntity(ctx, e)`, 1},
		{"field named Recreator", "p", "", `_ = svc.Recreator`, 0},
		{"unrelated bare name outside package", "p", "", `RecreateEntity(ctx, e)`, 0},
		{"ordinary create", "p", imp, `m.CreateEntity(ctx, e, opts); _ = entitymanager.New`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "package " + tc.pkg + "\n" + tc.imports + "\nfunc f() {\n" + tc.body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := recreateEntryPoints(fset, file); len(got) != tc.want {
				t.Errorf("entry points = %v, want %d", got, tc.want)
			}
		})
	}
}

// Declarations are not references: the function, the adapter type, its
// method and a consumer's interface method all declare the name.
func TestRecreateEntryPoints_DeclarationsAreNotFindings(t *testing.T) {
	t.Parallel()
	src := `package entitymanager
func RecreateEntity() {}
type Recreator struct{}
func (Recreator) RecreateEntity() {}
type iface interface{ RecreateEntity() }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := recreateEntryPoints(fset, file); len(got) != 0 {
		t.Errorf("entry points = %v, want none", got)
	}
}
