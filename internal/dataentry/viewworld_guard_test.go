package dataentry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestExecuteViewCallersDeclareTheirWorld is the guard [viewWorld]'s doc
// promises, and the reason the world is a parameter rather than a ctx read.
//
// # What it protects
//
// executeView is a SHARED ENGINE with three callers, only one of which is
// world-capable. The other two — `_sidepanel` and the command runner — must
// pass defaultViewWorld() EXPLICITLY. If a future caller (or a refactor of an
// existing one) forgets, the engine would still compile, and the surface would
// inherit whatever world the argument happened to carry.
//
// The command path is why this is a guard rather than a convention: its
// viewResult is marshaled to JSON and piped to an operator shell script's
// stdin, so a world applied there changes what an EXTERNAL PROCESS receives,
// past any layer that could observe it.
//
// # Why it scans for the argument rather than counting callers
//
// A test that merely counted call sites would pass when someone changed one
// from defaultViewWorld() to viewWorldFromRequest() — the exact regression it
// exists to catch. So it reads the fourth argument of every executeView call
// and requires it to name a world constructor.
//
// Mutation-checked: swapping either unscoped call site to
// viewWorldFromRequest() fails this test.
func TestExecuteViewCallersDeclareTheirWorld(t *testing.T) {
	t.Parallel()

	// The ONE file permitted to pass a request-derived world. Anything else
	// must pass defaultViewWorld().
	worldCapableFile := "views_handler.go"

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	var checked int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, name, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "executeView" {
				return true
			}
			checked++
			if len(call.Args) != 4 {
				t.Errorf("%s: executeView called with %d args, want 4 — the "+
					"world must be passed explicitly", name, len(call.Args))
				return true
			}
			got := worldArgName(call.Args[3])
			switch got {
			case "defaultViewWorld":
				// Always fine: this surface declares it serves the default world.
			case "viewWorldFromRequest":
				if name != worldCapableFile {
					t.Errorf("%s passes a REQUEST-derived world to executeView, but "+
						"only %s is world-capable. executeView is shared with the "+
						"side panel and the command runner — and the command runner "+
						"pipes its result to an external process, where a world "+
						"cannot be observed downstream. Pass defaultViewWorld() "+
						"unless this surface has been scoped and tested for worlds.",
						name, worldCapableFile)
				}
			default:
				t.Errorf("%s: executeView's world argument is %q, which names neither "+
					"defaultViewWorld() nor viewWorldFromRequest(). A surface must "+
					"DECLARE its world at the call site so a reader sees a choice "+
					"rather than an omission.", name, got)
			}
			return true
		})
	}

	// A guard that scanned nothing is not a guard: a rename would otherwise
	// make this pass silently forever.
	if checked < 3 {
		t.Fatalf("found only %d executeView call sites, expected at least 3 "+
			"(_views, _sidepanel, command runner). If they were renamed or "+
			"removed, update this guard rather than letting it go quiet", checked)
	}
}

// worldArgName renders the callee name of a world argument, or a description
// of why it is not a plain constructor call.
func worldArgName(arg ast.Expr) string {
	call, ok := arg.(*ast.CallExpr)
	if !ok {
		if ident, isIdent := arg.(*ast.Ident); isIdent {
			return ident.Name
		}
		return "<not a constructor call>"
	}
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if fn.Sel != nil {
			return fn.Sel.Name
		}
	}
	return "<unrecognized>"
}
