package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// execEnvGuard is the child-environment rule's failure vocabulary
// (GH-2242). A child started without Env inherits the whole process
// environment, including RELA_TOKEN_KEY, RELA_DATABASE_URL and every
// REMOTE_CREDENTIAL_ variable. cmdexec.Environ leaves those out.
var execEnvGuard = guard{
	what:   "exec.Command without Env",
	list:   "execEnvAllowlist",
	advice: "set Env on the command, from cmdexec.Environ or a slice built on it",
}

// execEnvAllowlist pins the exec.Command calls whose function never sets
// Env. It is empty and may only stay so.
var execEnvAllowlist = map[string]allowed{}

// execWithoutEnv returns the position of every exec.Command or
// exec.CommandContext call in a function that assigns no .Env field.
func execWithoutEnv(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var calls []token.Pos
		setsEnv := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if isExecCommand(n) {
					calls = append(calls, n.Pos())
				}
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Env" {
						setsEnv = true
					}
				}
			}
			return true
		})
		if !setsEnv {
			for _, p := range calls {
				found = append(found, fset.Position(p))
			}
		}
	}
	return found
}

// isExecCommand reports whether call is exec.Command or exec.CommandContext.
func isExecCommand(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "exec" {
		return false
	}
	return sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext"
}

func TestExecCommandsSetEnv(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, execWithoutEnv)
	checkAllowlist(t, execEnvGuard, got, execEnvAllowlist)
}

func TestExecWithoutEnv(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no env", `c := exec.Command("x"); _ = c.Run()`, 1},
		{"context no env", `_, _ = exec.CommandContext(ctx, "x").Output()`, 1},
		{"env set", `c := exec.Command("x"); c.Env = cmdexec.Environ(); _ = c.Run()`, 0},
		{"two calls no env", `exec.Command("a"); exec.Command("b")`, 2},
		{"other package", `c := other.Command("x"); _ = c`, 0},
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
			if got := execWithoutEnv(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
