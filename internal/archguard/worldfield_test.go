package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// worldFieldGuard is the vocabulary of the worldfield rule.
var worldFieldGuard = guard{
	what: "lua.ReadDeps or mcp.Deps literal without World",
	list: "worldFieldAllowlist",
	advice: "set World to the schema's default world (worlds.Compiled.DefaultWorld). An unset world " +
		"fails closed at construction, and a world left out of a literal is the RR-HKVULG defect: " +
		"the reads then run in a world nobody chose (TKT-7IZHP0 design §11)",
}

// worldFieldTypes are the dependency bundles whose literals must name World,
// keyed by the package that declares them and then by type name.
var worldFieldTypes = map[string]string{
	"lua": "ReadDeps",
	"mcp": "Deps",
}

// worldFieldLiterals returns the position of every lua.ReadDeps or mcp.Deps
// composite literal that has elements but no `World:` key.
//
// The empty literal is skipped: it is the zero value returned beside an
// error, and the constructors refuse it (mcp.NewServer and lua's runtime
// both reject an unset world), so it cannot reach a read.
//
// The type is matched by name. Outside the declaring package that is the
// selector `<import>.ReadDeps`, whatever the import is called; inside it is
// the bare identifier. An import alias does not hide a literal, and an
// unrelated package's `Deps` type is not matched by the selector rule unless
// its import name happens to be lua or mcp, which the scan tolerates as a
// false positive.
func worldFieldLiterals(fset *token.FileSet, file *ast.File) []token.Position {
	pkg := file.Name.Name
	imports := map[string]string{} // local import name -> declaring package
	for _, imp := range file.Imports {
		path := imp.Path.Value[1 : len(imp.Path.Value)-1]
		base := filepath.Base(path)
		if _, ok := worldFieldTypes[base]; !ok || filepath.Base(filepath.Dir(path)) != "internal" {
			continue
		}
		name := base
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = base
	}
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || len(lit.Elts) == 0 || !isWorldFieldType(lit.Type, pkg, imports) {
			return true
		}
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "World" {
					return true
				}
			}
		}
		found = append(found, fset.Position(lit.Lbrace))
		return true
	})
	return found
}

// isWorldFieldType reports whether expr names one of worldFieldTypes, from
// inside the declaring package or through an import of it.
func isWorldFieldType(expr ast.Expr, pkg string, imports map[string]string) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return worldFieldTypes[pkg] == t.Name
	case *ast.SelectorExpr:
		x, ok := t.X.(*ast.Ident)
		if !ok {
			return false
		}
		decl, ok := imports[x.Name]
		return ok && worldFieldTypes[decl] == t.Sel.Name
	}
	return false
}

// TestNoWorldlessDeps pins every World-less lua.ReadDeps and mcp.Deps
// literal to worldFieldAllowlist, exactly.
func TestNoWorldlessDeps(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, worldFieldLiterals)
	checkAllowlist(t, worldFieldGuard, got, worldFieldAllowlist)
}

func TestWorldFieldAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, worldFieldAllowlist)
}

func TestWorldFieldLiterals(t *testing.T) {
	t.Parallel()
	const imports = `import (
	"github.com/Sourcehaven-BV/rela/internal/lua"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	other "example.com/x/lua"
)
`
	cases := []struct {
		name string
		pkg  string
		body string
		want int
	}{
		{"ReadDeps without World", "p", `_ = lua.ReadDeps{Meta: m}`, 1},
		{"ReadDeps with World", "p", `_ = lua.ReadDeps{Meta: m, World: w}`, 0},
		{"aliased mcp Deps without World", "p", `_ = relamcp.Deps{Store: s}`, 1},
		{"aliased mcp Deps with World", "p", `_ = relamcp.Deps{Store: s, World: w}`, 0},
		{"empty literal is the zero value beside an error", "p", `_ = relamcp.Deps{}`, 0},
		{"nested in WriteDeps", "p", `_ = lua.WriteDeps{ReadDeps: lua.ReadDeps{Meta: m}}`, 1},
		{"pointer literal", "p", `_ = &lua.ReadDeps{Meta: m}`, 1},
		{"bare name inside package lua", "lua", `_ = ReadDeps{Meta: m}`, 1},
		{"bare name in another package", "p", `_ = ReadDeps{Meta: m}`, 0},
		{"a package not under internal", "p", `_ = other.ReadDeps{Meta: m}`, 0},
		{"another type of the same package", "p", `_ = lua.WriteDeps{EntityManager: e}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "package " + tc.pkg + "\n" + imports + "func f() {\n" + tc.body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := worldFieldLiterals(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
