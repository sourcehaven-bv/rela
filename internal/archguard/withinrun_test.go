package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// withinRunGuard is the flow edit exemption rule's failure vocabulary
// (TKT-XI7IDBS). flowedits.WithinRun lifts a run's outside_edits from a
// write. It is a capability: a caller that sets it from request data lets a
// user write a property the flow withholds. Only the flow runtime's own
// writes may set it.
var withinRunGuard = guard{
	what: "flowedits.WithinRun reference",
	list: "withinRunAllowlist",
	advice: "only a run's own step write and output write may lift its outside_edits; " +
		"a write from anywhere else must stay narrowed",
}

// withinRunAllowlist pins the references to flowedits.WithinRun. It may
// only shrink.
var withinRunAllowlist = map[string]allowed{
	"internal/appbuild/flowruntime.go": {1, "flowOutputs.Set: the run's own set output"},
	"internal/appbuild/flowsteps.go":   {1, "FlowControl.complete: the step write of the run being completed"},
}

// withinRunRefs returns the position of every qualified reference to
// WithinRun, called or taken as a value.
func withinRunRefs(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithinRun" {
			found = append(found, fset.Position(sel.Sel.Pos()))
		}
		return true
	})
	return found
}

func TestFlowEditsWithinRunCallers(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, withinRunRefs)
	checkAllowlist(t, withinRunGuard, got, withinRunAllowlist)
}

func TestWithinRunAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, withinRunAllowlist)
}

func TestWithinRunRefs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"call", `ctx = flowedits.WithinRun(ctx, id)`, 1},
		{"value", `f := flowedits.WithinRun; _ = f`, 1},
		{"other", `ctx = flowedits.Other(ctx)`, 0},
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
			if got := withinRunRefs(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
