package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The element range read (store.ElementScope, TKT-IQU106C) is a raw
// capability: it applies whatever scope its caller hands it, so a hand-built
// allow-all scope would bypass the row gate and the field gate. The only
// sound caller compiles the scope from the reader's ACL request at the
// request edge. This guard pins every use outside internal/store to
// elementRangeAllowlist, so a new caller is a reviewed decision.

// elementRangeAllowlist pins the uses of the element range reader outside
// internal/store. It is empty until the reconcile endpoint (step 5 of
// FEAT-CMVZQFI) adds its wiring and its handler, each with a reason.
var elementRangeAllowlist = map[string]allowed{}

var elementRangeGuard = guard{
	what: "element range read",
	list: "elementRangeAllowlist",
	advice: "build the store.ElementScope from the reader's bound acl.Request, never by hand, " +
		"and add the file to the allowlist with the reason it is a genuine edge",
}

// elementRangeUses returns the position of every call or reference to the
// range methods (RangeFingerprint, ListRange) and the reader constructors
// (pgstore.NewElementRanges, sqlitestore.NewElementRanges, elementnaive.New).
// The match is syntactic, by selector name.
func elementRangeUses(fset *token.FileSet, file *ast.File) []token.Position {
	var uses []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "RangeFingerprint", "ListRange", "NewElementRanges":
			uses = append(uses, fset.Position(sel.Sel.Pos()))
		case "New":
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "elementnaive" {
				uses = append(uses, fset.Position(sel.Sel.Pos()))
			}
		}
		return true
	})
	return uses
}

func TestNoUnlistedElementRangeReads(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, []string{"internal", "cmd"}, elementRangeUses)
	for path := range got {
		if strings.HasPrefix(path, "internal/store/") {
			delete(got, path) // the implementations and their reference
		}
	}
	checkAllowlist(t, elementRangeGuard, got, elementRangeAllowlist)
}

func TestElementRangeAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, elementRangeAllowlist)
}

func TestElementRangeUses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"fingerprint", `g.RangeFingerprint(ctx, scope, r)`, 1},
		{"list", `g.ListRange(ctx, scope, r, l)`, 1},
		{"method value", `f := g.ListRange; _ = f`, 1},
		{"pg reader", `pgstore.NewElementRanges(s)`, 1},
		{"naive reader", `elementnaive.New(s)`, 1},
		{"other New", `memstore.New()`, 0},
		{"other method", `st.ListRelations(ctx, q)`, 0},
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
			if got := elementRangeUses(fset, file); len(got) != tc.want {
				t.Errorf("uses = %v, want %d", got, tc.want)
			}
		})
	}
}
