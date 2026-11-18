package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// pgxConnectGuard is the direct-connection rule's failure vocabulary
// (BUG-A8GG1CD). The configured DSN is a pool DSN and may carry pool-only
// keys such as pool_max_conns; pgx.Connect sends those to the server, which
// refuses the connection.
var pgxConnectGuard = guard{
	what: "pgx.Connect call",
	list: "pgxConnectAllowlist",
	advice: "use pgstore.ConnectDirect, which strips pgxpool-only keys before it connects, " +
		"or pgx.ConnectConfig on a config from pgxpool.ParseConfig",
}

// pgxConnectAllowlist pins the non-test pgx.Connect calls. It may only
// shrink, and it is empty.
var pgxConnectAllowlist = map[string]allowed{}

// pgxConnectCalls returns the position of every `pgx.Connect(...)` call.
// The match is on the package name pgx, so a renamed import is not caught.
func pgxConnectCalls(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Connect" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "pgx" {
			found = append(found, fset.Position(sel.Sel.Pos()))
		}
		return true
	})
	return found
}

func TestNoDirectPgxConnect(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, pgxConnectCalls)
	checkAllowlist(t, pgxConnectGuard, got, pgxConnectAllowlist)
}

func TestPgxConnectCalls(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"connect", `pgx.Connect(ctx, dsn)`, 1},
		{"connect config", `pgx.ConnectConfig(ctx, cfg)`, 0},
		{"other package", `pgstore.Connect(ctx, dsn)`, 0},
		{"method value", `f := pgx.Connect; _ = f`, 1},
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
			if got := pgxConnectCalls(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
