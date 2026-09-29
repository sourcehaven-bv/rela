package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// bareRefGuard is the bare-ref rule's failure vocabulary.
var bareRefGuard = guard{
	what: "bare entity.Ref",
	list: "bareRefAllowlist",
	advice: "name the face the read or write means: the entity's own Ref() for a row you hold, " +
		"the resolved face from visibility.Resolver, or the parsed face of a user address " +
		"(cli address helper, entity.ParseRef). A faced type stores no row at the zero face " +
		"(DEC-NPZICR), so Ref{ID: id} silently finds nothing for it. " +
		"If you only moved an existing literal between files, move its allowlist entry with it",
}

// bareRefs returns the position of every entity.Ref composite literal that
// names an id but no face: it sets ID and either omits Face or sets it to an
// empty string literal (bare or converted, entity.Face("")).
//
// The match is syntactic, like the other guards here. A literal is a Ref when
// its type is an identifier `Ref` or a selector ending in `.Ref` (any import
// alias); entity.Ref is the only type of that name in the module. Not caught:
// a face variable that happens to be empty, a positional literal, and
// entity.ParseRef of a bare id handed to the store. Review covers those; the
// CLI address helper and the resolver are the sanctioned parsers.
func bareRefs(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isRefType(lit.Type) {
			return true
		}
		face, hasFace := keyValue(lit, "Face")
		if _, hasID := keyValue(lit, "ID"); hasID && (!hasFace || isEmptyString(face)) {
			found = append(found, fset.Position(lit.Pos()))
		}
		return true
	})
	return found
}

func isRefType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "Ref"
	case *ast.SelectorExpr:
		return t.Sel.Name == "Ref"
	}
	return false
}

// keyValue returns the value lit sets for key, if it sets one.
func keyValue(lit *ast.CompositeLit, key string) (ast.Expr, bool) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
			return kv.Value, true
		}
	}
	return nil, false
}

// isEmptyString reports whether e is an empty string literal, bare or wrapped
// in a one-argument conversion such as entity.Face("").
func isEmptyString(e ast.Expr) bool {
	if conv, ok := e.(*ast.CallExpr); ok && len(conv.Args) == 1 {
		e = conv.Args[0]
	}
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && (lit.Value == `""` || lit.Value == "``")
}

// TestNoNewBareRefs pins every bare entity.Ref literal in non-test code to
// bareRefAllowlist, exactly.
func TestNoNewBareRefs(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, bareRefs)
	checkAllowlist(t, bareRefGuard, got, bareRefAllowlist)
}

func TestBareRefAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, bareRefAllowlist)
}

func TestBareRefs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"id only", `r := entity.Ref{ID: id}; _ = r`, 1},
		{"aliased package", `r := entityPkg.Ref{ID: id}; _ = r`, 1},
		{"unqualified", `r := Ref{ID: id}; _ = r`, 1},
		{"empty face literal", `r := entity.Ref{ID: id, Face: ""}; _ = r`, 1},
		{"converted empty face", `r := entity.Ref{ID: id, Face: entity.Face("")}; _ = r`, 1},
		{"named face", `r := entity.Ref{ID: id, Face: face}; _ = r`, 0},
		{"literal face", `r := entity.Ref{ID: id, Face: "draft"}; _ = r`, 0},
		{"zero value", `r := entity.Ref{}; _ = r`, 0},
		{"face only", `r := entity.Ref{Face: face}; _ = r`, 0},
		{"in a call", `st.GetEntity(ctx, entity.Ref{ID: id})`, 1},
		{"other type", `r := store.RelationKey{ID: id}; _ = r`, 0},
		{"entity's own ref", `st.GetEntity(ctx, e.Ref())`, 0},
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
			if got := bareRefs(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}

// The guard must fail on a synthetic violation end to end, and skip tests,
// fixtures, the excluded trees and the entity package itself.
func TestScanTree_FindsSyntheticBareRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const bad = "func f() { _ = entity.Ref{ID: id} }\n"
	for rel, pkg := range map[string]string{
		"internal/pkg/bad.go":           "pkg",
		"internal/pkg/bad_test.go":      "pkg",
		"internal/pkg/testdata/x.go":    "x",
		"internal/store/storetest/h.go": "storetest",
		"internal/entity/ref.go":        "entity",
		"internal/_scratch/y.go":        "y",
		"cmd/tool/main.go":              "main",
	} {
		if err := mkdirAllWrite(filepath.Join(root, filepath.FromSlash(rel)), "package "+pkg+"\n"+bad); err != nil {
			t.Fatal(err)
		}
	}
	got := scanTree(t, root, scannedRoots, bareRefs)
	if len(got) != 2 || len(got["internal/pkg/bad.go"]) != 1 || len(got["cmd/tool/main.go"]) != 1 {
		t.Fatalf("scan = %v, want one finding in each of internal/pkg/bad.go and cmd/tool/main.go", got)
	}
	msgs := diffAllowlist(bareRefGuard, got, map[string]int{"cmd/tool/main.go": 1})
	if len(msgs) != 1 || !strings.HasPrefix(msgs[0], "internal/pkg/bad.go:") {
		t.Errorf("want one finding for the unlisted file; got %q", msgs)
	}
}
