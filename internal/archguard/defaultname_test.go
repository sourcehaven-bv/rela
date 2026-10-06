package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// defaultNameGuard is the vocabulary of the defaultname rule.
var defaultNameGuard = guard{
	what: "reference to the default world's name",
	list: "defaultNameAllowlist",
	advice: "ask worlds.Compiled instead (DefaultWorld, DefaultWorldName, Lookup, Names). The name " +
		"\"default\" means a world only when the schema declares none; beside declared worlds it " +
		"names nothing, so a site that compares against the literal name treats a project with " +
		"worlds as if it had the generated one (TKT-7IZHP0 design §11, D11)",
}

// defaultNameIdents are the identifiers that spell the default world's name:
// metamodel.DefaultWorldName, acl.DefaultWorldName and dataentry's private
// defaultWorldName.
var defaultNameIdents = map[string]bool{
	"DefaultWorldName": true,
	"defaultWorldName": true,
}

// defaultNameHomes are the files and trees that may name the default world:
// the worlds compiler that owns the answer, the schema and config validators
// that reject the name where it means nothing, and the ACL grant syntax that
// declares its own copy. A path ending in "/" is a tree.
var defaultNameHomes = []string{
	"internal/worlds/",
	"internal/metamodel/",
	"internal/dataentryconfig/validate.go",
	"internal/dataentryconfig/nextaction_validate.go",
	"internal/acl/worldgrant.go",
}

// defaultNameRefs returns the position of every identifier named
// DefaultWorldName or defaultWorldName, whether a qualified reference, a
// package-local one or the declaration itself.
func defaultNameRefs(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && defaultNameIdents[id.Name] {
			found = append(found, fset.Position(id.Pos()))
		}
		return true
	})
	return found
}

// outsideHomes drops the findings in defaultNameHomes.
func outsideHomes(got map[string][]token.Position) map[string][]token.Position {
	out := map[string][]token.Position{}
	for path, refs := range got {
		if !inDefaultNameHome(path) {
			out[path] = refs
		}
	}
	return out
}

func inDefaultNameHome(path string) bool {
	for _, home := range defaultNameHomes {
		if path == home || (strings.HasSuffix(home, "/") && strings.HasPrefix(path, home)) {
			return true
		}
	}
	return false
}

// TestNoNewDefaultNameRefs pins every reference to the default world's name
// outside its homes to defaultNameAllowlist, exactly.
func TestNoNewDefaultNameRefs(t *testing.T) {
	t.Parallel()
	got := outsideHomes(scanTree(t, repoRoot, scannedRoots, defaultNameRefs))
	checkAllowlist(t, defaultNameGuard, got, defaultNameAllowlist)
}

func TestDefaultNameAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, defaultNameAllowlist)
	for path := range defaultNameAllowlist {
		if inDefaultNameHome(path) {
			t.Errorf("%s: allowlist entry inside a home; the guard never counts it", path)
		}
	}
}

func TestDefaultNameRefs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"qualified", `_ = metamodel.DefaultWorldName`, 1},
		{"other package", `_ = acl.DefaultWorldName`, 1},
		{"package-local", `_ = defaultWorldName`, 1},
		{"two in one comparison", `_ = x == acl.DefaultWorldName || y == defaultWorldName`, 2},
		{"the literal is not the identifier", `_ = "default"`, 0},
		{"a different name", `_ = metamodel.DefaultWorld`, 0},
		{"a comment", "// metamodel.DefaultWorldName\n_ = 1", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "package p\nfunc f() {\n" + tc.body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := defaultNameRefs(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}

// The guard must skip the homes and tests.
func TestDefaultNameGuard_SkipsHomes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const ref = "var _ = metamodel.DefaultWorldName\n"
	for rel, pkg := range map[string]string{
		"internal/worlds/w.go":                   "worlds",
		"internal/metamodel/m.go":                "metamodel",
		"internal/dataentryconfig/validate.go":   "dataentryconfig",
		"internal/dataentryconfig/nextaction.go": "dataentryconfig",
		"internal/dataentry/h.go":                "dataentry",
		"internal/dataentry/h_test.go":           "dataentry",
	} {
		if err := mkdirAllWrite(filepath.Join(root, filepath.FromSlash(rel)), "package "+pkg+"\n"+ref); err != nil {
			t.Fatal(err)
		}
	}
	got := outsideHomes(scanTree(t, root, []string{"internal"}, defaultNameRefs))
	if len(got) != 2 || len(got["internal/dataentry/h.go"]) != 1 || len(got["internal/dataentryconfig/nextaction.go"]) != 1 {
		t.Fatalf("scan = %v, want one finding in each of internal/dataentry/h.go and "+
			"internal/dataentryconfig/nextaction.go", got)
	}
}
