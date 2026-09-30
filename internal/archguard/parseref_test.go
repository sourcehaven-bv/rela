package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// parseRefGuard is the vocabulary of the parseref rule.
var parseRefGuard = guard{
	what: "entity.ParseRef call at an edge",
	list: "parseRefAllowlist",
	advice: "parse a caller's address with entity.ParseAddress and let visibility.Resolver turn it " +
		"into a Ref. entity.ParseRef reads a bare id as the implicit face, which is right for a " +
		"serialized stored key (cursor, audit, relation key) and wrong for a user address: a faced " +
		"type has no row at the implicit face (TKT-7IZHP0 design §2)",
}

// parseRefEdges are the repo-relative trees whose files take addresses from
// callers: HTTP handlers, MCP tools, Lua bindings and CLI commands.
var parseRefEdges = []string{"internal/dataentry/", "internal/mcp/", "internal/lua/", "internal/cli/"}

// parseRefCalls returns the position of every `x.ParseRef` selector, called
// or taken as a value, whatever x's import name.
func parseRefCalls(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "ParseRef" {
			if _, ok := sel.X.(*ast.Ident); ok {
				found = append(found, fset.Position(sel.Sel.Pos()))
			}
		}
		return true
	})
	return found
}

// edgeFindings keeps the findings in the parseRefEdges trees.
func edgeFindings(got map[string][]token.Position) map[string][]token.Position {
	out := map[string][]token.Position{}
	for path, reads := range got {
		for _, prefix := range parseRefEdges {
			if strings.HasPrefix(path, prefix) {
				out[path] = reads
				break
			}
		}
	}
	return out
}

// TestNoNewParseRefAtEdges pins every entity.ParseRef call in the edge trees
// to parseRefAllowlist, exactly.
func TestNoNewParseRefAtEdges(t *testing.T) {
	t.Parallel()
	got := edgeFindings(scanTree(t, repoRoot, scannedRoots, parseRefCalls))
	checkAllowlist(t, parseRefGuard, got, parseRefAllowlist)
}

func TestParseRefAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, parseRefAllowlist)
	for path := range parseRefAllowlist {
		inEdge := false
		for _, prefix := range parseRefEdges {
			inEdge = inEdge || strings.HasPrefix(path, prefix)
		}
		if !inEdge {
			t.Errorf("%s: allowlist entry outside the edge trees; the guard never scans it", path)
		}
	}
}

func TestParseRefCalls(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"call", `_, _ = entity.ParseRef(s)`, 1},
		{"aliased package", `_, _ = entityPkg.ParseRef(s)`, 1},
		{"method value", `f := entity.ParseRef; _ = f`, 1},
		{"address parse", `_, _ = entity.ParseAddress(s)`, 0},
		{"state ref parse", `_, _, _ = entity.ParseStateRef(s)`, 0},
		{"method on a value", `_, _ = x.y.ParseRef(s)`, 0},
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
			if got := parseRefCalls(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}

// The guard must scan only the edge trees, and skip tests.
func TestParseRefGuard_ScansOnlyEdges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const call = "func f() { _, _ = entity.ParseRef(s) }\n"
	for rel, pkg := range map[string]string{
		"internal/dataentry/h.go":      "dataentry",
		"internal/dataentry/h_test.go": "dataentry",
		"internal/cli/c.go":            "cli",
		"internal/store/s.go":          "store",
		"internal/audit/a.go":          "audit",
	} {
		if err := mkdirAllWrite(filepath.Join(root, filepath.FromSlash(rel)), "package "+pkg+"\n"+call); err != nil {
			t.Fatal(err)
		}
	}
	got := edgeFindings(scanTree(t, root, []string{"internal"}, parseRefCalls))
	if len(got) != 2 || len(got["internal/dataentry/h.go"]) != 1 || len(got["internal/cli/c.go"]) != 1 {
		t.Fatalf("scan = %v, want one finding in each of internal/dataentry/h.go and internal/cli/c.go", got)
	}
}
