package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// unselectedQueryGuard is the vocabulary of the EntityQuery selection rule.
var unselectedQueryGuard = guard{
	what: "EntityQuery literal without Faces",
	list: "unselectedQueryAllowlist",
	advice: "set Faces in the literal: store.InWorld(w) with the request's or the configured default world " +
		"when a reader chooses which face it sees, store.AllFaces() for a raw scan over every row, " +
		"store.AtFaces(fs...) for exact faces. The zero selection is ErrInvalidQuery on every backend " +
		"(TKT-KQXVF7). A literal completed by a helper before it runs may be listed with a reason",
}

// unselectedQueries returns the position of every EntityQuery composite
// literal (bare or qualified) that does not set the Faces key.
//
// The match is syntactic, like zeroFaceReads. Not caught: an EntityQuery
// built as a zero value (`var q store.EntityQuery`) or through new(); review
// is the backstop for those. A literal that sets Faces from a variable
// holding the zero selection is not caught either; the backends refuse it at
// run time.
func unselectedQueries(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && isEntityQuery(lit.Type) && !setsKey(lit, "Faces") {
			found = append(found, fset.Position(lit.Pos()))
		}
		return true
	})
	return found
}

// TestNoNewUnselectedEntityQueries pins every EntityQuery literal without a
// face selection to unselectedQueryAllowlist, exactly.
func TestNoNewUnselectedEntityQueries(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, unselectedQueries)
	counts := make(map[string]int, len(unselectedQueryAllowlist))
	for path, e := range unselectedQueryAllowlist {
		counts[path] = e.n
	}
	for _, msg := range diffAllowlist(unselectedQueryGuard, got, counts) {
		t.Error(msg)
	}
}

// unselectedGraphQueryGuard is the vocabulary of the GraphQuery selection
// rule. A zero GraphQuery selection fails at run time as ErrInvalidQuery,
// which a user sees as a 500, so a new literal must say which faces it reads.
var unselectedGraphQueryGuard = guard{
	what: "GraphQuery literal without Faces",
	list: "unselectedGraphQueryAllowlist",
	advice: "set Faces in the literal, as for EntityQuery. An ACL template or a literal a helper " +
		"completes before it runs (a stampScope caller, a \"no query\" return) may be listed with a reason",
}

// unselectedGraphQueries returns the position of every GraphQuery composite
// literal that does not set the Faces key. Syntactic, with the same blind
// spots as unselectedQueries.
func unselectedGraphQueries(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && isNamed(lit.Type, "GraphQuery") && !setsKey(lit, "Faces") {
			found = append(found, fset.Position(lit.Pos()))
		}
		return true
	})
	return found
}

// isNamed reports whether e names the type name, bare or package-qualified.
func isNamed(e ast.Expr, name string) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == name
	case *ast.SelectorExpr:
		return t.Sel.Name == name
	}
	return false
}

// TestNoNewUnselectedGraphQueries pins every GraphQuery literal without a
// face selection to unselectedGraphQueryAllowlist, exactly.
func TestNoNewUnselectedGraphQueries(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, unselectedGraphQueries)
	counts := make(map[string]int, len(unselectedGraphQueryAllowlist))
	for path, e := range unselectedGraphQueryAllowlist {
		counts[path] = e.n
	}
	for _, msg := range diffAllowlist(unselectedGraphQueryGuard, got, counts) {
		t.Error(msg)
	}
}

// trivialScopeGuard is the vocabulary of the TrivialScope rule.
var trivialScopeGuard = guard{
	what: "store.TrivialScope call",
	list: "trivialScopeAllowlist",
	advice: "take the world from the request (dataentry worldScopeFrom), the wiring " +
		"(worlds.Compiled.Default, lua.ReadDeps.World, mcp.Deps.World, appbuild) or the caller. " +
		"A hard-coded trivial scope is not the configured default world once a project declares " +
		"default_world (TKT-7IZHP0), and every call site would have to be found again",
}

// trivialScopeExempt are the repo-relative trees that own the trivial scope:
// the store defines it and internal/worlds compiles the configured worlds from it.
var trivialScopeExempt = []string{"internal/store/", "internal/worlds/"}

// trivialScopeCalls returns the position of every `x.TrivialScope` selector,
// called or taken as a value, whatever x's import name. It matches the way
// parseRefCalls does, so an aliased store import cannot slip past it.
func trivialScopeCalls(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "TrivialScope" {
			if _, ok := sel.X.(*ast.Ident); ok {
				found = append(found, fset.Position(sel.Sel.Pos()))
			}
		}
		return true
	})
	return found
}

// TestNoNewTrivialScopeCalls pins every store.TrivialScope call outside the
// packages that own it to trivialScopeAllowlist, exactly.
func TestNoNewTrivialScopeCalls(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, trivialScopeCalls)
	for path := range got {
		for _, prefix := range trivialScopeExempt {
			if strings.HasPrefix(path, prefix) {
				delete(got, path)
			}
		}
	}
	counts := make(map[string]int, len(trivialScopeAllowlist))
	for path, e := range trivialScopeAllowlist {
		counts[path] = e.n
	}
	for _, msg := range diffAllowlist(trivialScopeGuard, got, counts) {
		t.Error(msg)
	}
}

// Every entry must say why: the allowlists are the record of what is still
// to move and what is deliberate.
func TestFaceSelectionAllowlists_HaveReasons(t *testing.T) {
	t.Parallel()
	for name, list := range map[string]map[string]allowed{
		"unselectedQueryAllowlist":      unselectedQueryAllowlist,
		"unselectedGraphQueryAllowlist": unselectedGraphQueryAllowlist,
		"trivialScopeAllowlist":         trivialScopeAllowlist,
	} {
		for path, e := range list {
			if strings.TrimSpace(e.reason) == "" {
				t.Errorf("%s: %s: entry has no reason", name, path)
			}
			if e.n <= 0 {
				t.Errorf("%s: %s: count %d; delete the entry instead", name, path, e.n)
			}
		}
	}
}

func TestUnselectedGraphQueries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"qualified, no Faces", `_ = store.GraphQuery{EntityType: "x"}`, 1},
		{"bare, no Faces", `_ = GraphQuery{}`, 1},
		{"with Faces", `_ = store.GraphQuery{EntityType: "x", Faces: store.AllFaces()}`, 0},
		{"other type", `_ = store.EntityQuery{Type: "x"}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := unselectedGraphQueries(parseBody(t, tc.body)); len(got) != tc.want {
				t.Errorf("found = %v, want %d", got, tc.want)
			}
		})
	}
}

func TestUnselectedQueries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"qualified, no Faces", `_ = store.EntityQuery{Type: "x"}`, 1},
		{"bare, no Faces", `_ = EntityQuery{}`, 1},
		{"with Faces", `_ = store.EntityQuery{Type: "x", Faces: store.AllFaces()}`, 0},
		{"FaceIn is not Faces", `_ = store.EntityQuery{FaceIn: fs}`, 1},
		{"other type", `_ = store.GraphQuery{EntityType: "x"}`, 0},
		{"nested", `f(store.EntityQuery{IDs: ids})`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := unselectedQueries(parseBody(t, tc.body)); len(got) != tc.want {
				t.Errorf("found = %v, want %d", got, tc.want)
			}
		})
	}
}

func TestTrivialScopeCalls(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"call", `_ = store.InWorld(store.TrivialScope())`, 1},
		{"method value", `f := store.TrivialScope; _ = f`, 1},
		{"aliased package", `_ = storepkg.TrivialScope()`, 1},
		{"method on a value", `_ = x.y.TrivialScope()`, 0},
		{"compiled default", `_ = compiled.Default()`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := trivialScopeCalls(parseBody(t, tc.body)); len(got) != tc.want {
				t.Errorf("found = %v, want %d", got, tc.want)
			}
		})
	}
}

func parseBody(t *testing.T, body string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", "package p\nfunc f() {\n"+body+"\n}\n", 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return fset, file
}
