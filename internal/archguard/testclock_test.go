package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// wallClockGuard is the clock rule's failure vocabulary.
var wallClockGuard = guard{
	what: "wall-clock read",
	list: "wallClockAllowlist",
	advice: "take the clock from the caller: automation.WithClock, entitymanager.Deps.Clock, " +
		"writerule.WithClock, predicatefns.NewEvaluatorWithClock, computed.CompileWithClock, " +
		"memstore.WithClock or a lease.Clock. appbuild.WithClock reaches all of them, and a test " +
		"world that pins it must see the pinned time everywhere. Audit and version stamps are the " +
		"exception; allowlist those with a reason",
}

// wallClockRoots are the write-path packages whose time must follow
// appbuild.WithClock.
var wallClockRoots = []string{
	"internal/automation",
	"internal/entitymanager",
	"internal/validation",
	"internal/writerule",
	"internal/scopes",
	"internal/computed",
	"internal/lua",
	"internal/store/memstore",
	"internal/lease",
	"internal/predicatefns",
	// The appbuild files that read the time on behalf of a world.
	"internal/appbuild/softdeletegc.go",
	"internal/appbuild/scheduled_mail.go",
}

// wallClockReads returns the position of every time.Now selector: a call and
// a function value (a default clock) both count. The match is syntactic and
// assumes the time package is imported as "time".
func wallClockReads(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Now" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "time" {
			found = append(found, fset.Position(sel.Pos()))
		}
		return true
	})
	return found
}

// TestWallClockReads pins every wall-clock read in the write-path packages
// to wallClockAllowlist.
func TestWallClockReads(t *testing.T) {
	checkReasons(t, wallClockAllowlist)
	got := scanTree(t, repoRoot, wallClockRoots, wallClockReads)
	checkAllowlist(t, wallClockGuard, got, wallClockAllowlist)
}

// TestWallClockReads_Matcher pins what the matcher counts on synthetic
// source. TestDiffAllowlist covers how a count above the allowlist fails.
func TestWallClockReads_Matcher(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"call", "package p\nimport \"time\"\nvar _ = time.Now()", 1},
		{"function value", "package p\nimport \"time\"\nvar f = time.Now", 1},
		{"method on a value is not the package", "package p\ntype c struct{}\nfunc (c) Now() int { return 0 }\nvar _ = c{}.Now()", 0},
		{"comment does not count", "package p\n// time.Now()\nvar x = 1", 0},
		{"two", "package p\nimport \"time\"\nvar a, b = time.Now(), time.Now()", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", tc.src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(wallClockReads(fset, file)); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}
