package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// assertedByGuard is the asserted-by rule's failure vocabulary.
var assertedByGuard = guard{
	what: "audit.Record missing AssertedBy",
	list: "the empty allowlist",
	advice: "add AssertedBy: audit.AssertedByFrom(ctx) next to Principal. A local command run with " +
		"--run-as acts as a self-asserted job identity, and asserted_by is the only record of the " +
		"OS user who asserted it; without it the audit names nobody who can be asked",
}

// recordsWithoutAssertedBy returns the position of every audit.Record
// composite literal that sets Principal but not AssertedBy. A literal is an
// audit.Record when its type is a selector `audit.Record`, or the identifier
// `Record` inside package audit. The match is syntactic: a Record built
// field by field after the literal is not caught.
func recordsWithoutAssertedBy(fset *token.FileSet, file *ast.File) []token.Position {
	inAudit := file.Name.Name == "audit"
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isAuditRecord(lit.Type, inAudit) {
			return true
		}
		_, hasPrincipal := keyValue(lit, "Principal")
		if _, hasAsserted := keyValue(lit, "AssertedBy"); hasPrincipal && !hasAsserted {
			found = append(found, fset.Position(lit.Pos()))
		}
		return true
	})
	return found
}

func isAuditRecord(e ast.Expr, inAudit bool) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return inAudit && t.Name == "Record"
	case *ast.SelectorExpr:
		x, ok := t.X.(*ast.Ident)
		return ok && x.Name == "audit" && t.Sel.Name == "Record"
	}
	return false
}

// TestAuditRecordsCarryAssertedBy fails on any audit.Record literal in
// non-test code that names a principal but drops the asserting OS user.
func TestAuditRecordsCarryAssertedBy(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, recordsWithoutAssertedBy)
	checkAllowlist(t, assertedByGuard, got, nil)
}

func TestRecordsWithoutAssertedBy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pkg  string
		body string
		want int
	}{
		{"principal only", "p", `sink.Record(audit.Record{Op: op, Principal: p})`, 1},
		{"both", "p", `sink.Record(audit.Record{Principal: p, AssertedBy: audit.AssertedByFrom(ctx)})`, 0},
		{"no principal", "p", `sink.Record(audit.Record{Op: op})`, 0},
		{"pointer", "p", `r := &audit.Record{Principal: p}; _ = r`, 1},
		{"inside package audit", "audit", `r := Record{Principal: p}; _ = r`, 1},
		{"Record outside audit", "p", `r := Record{Principal: p}; _ = r`, 0},
		{"other package", "p", `r := lease.Record{Principal: p}; _ = r`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "package " + tc.pkg + "\nfunc f() {\n" + tc.body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := recordsWithoutAssertedBy(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
