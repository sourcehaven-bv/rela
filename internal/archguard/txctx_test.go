package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// txCtxGuard is the transaction-context rule's failure vocabulary
// (TKT-VO6VG9). A version tag write inside a store transaction would run on
// a second connection that cannot see the transaction's writes (postgres) or
// would wait on its lock (sqlite). The store refuses it when ctx carries
// store.ContextInTx, which only works if every Tx callback marks the ctx it
// passes on.
var txCtxGuard = guard{
	what: "unmarked Tx callback",
	list: "txCtxAllowlist",
	advice: "derive the callback's ctx with store.ContextInTx(ctx) and pass that on, so a version " +
		"tag write inside the transaction is refused rather than run outside it",
}

// unmarkedTxCallbacks returns the position of every `x.Tx(ctx, fn)` call
// whose callback does not call ContextInTx. A callback that is not a
// function literal (a named func value) cannot be checked syntactically and
// counts as a finding.
//
// The match is syntactic: a callback that calls ContextInTx and then passes
// the outer ctx anyway is not caught. Review is the backstop for that.
func unmarkedTxCallbacks(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Tx" {
			return true
		}
		lit, ok := call.Args[1].(*ast.FuncLit)
		if !ok || !callsContextInTx(lit.Body) {
			found = append(found, fset.Position(sel.Sel.Pos()))
		}
		return true
	})
	return found
}

// callsContextInTx reports whether body calls ContextInTx, qualified or not.
func callsContextInTx(body *ast.BlockStmt) bool {
	marked := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !marked
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			marked = marked || fn.Sel.Name == "ContextInTx"
		case *ast.Ident:
			marked = marked || fn.Name == "ContextInTx"
		}
		return !marked
	})
	return marked
}

func TestNoUnmarkedTxCallbacks(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, unmarkedTxCallbacks)
	checkAllowlist(t, txCtxGuard, got, txCtxAllowlist)
}

func TestTxCtxAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, txCtxAllowlist)
}

func TestUnmarkedTxCallbacks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"marked", `st.Tx(ctx, func(v store.Store) error { return f(store.ContextInTx(ctx), v) })`, 0},
		{"marked in a nested statement", `st.Tx(ctx, func(v store.Store) error {
			if ok { c := store.ContextInTx(ctx); return f(c, v) }
			return nil
		})`, 0},
		{"unmarked", `st.Tx(ctx, func(v store.Store) error { return f(ctx, v) })`, 1},
		{"named callback", `st.Tx(ctx, rename)`, 1},
		{"other arity", `st.Tx(ctx)`, 0},
		{"other method", `st.Run(ctx, func() error { return nil })`, 0},
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
			if got := unmarkedTxCallbacks(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
