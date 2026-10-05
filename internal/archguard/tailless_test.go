package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// taillessKeyGuard is the tail-less relation key rule's failure vocabulary.
var taillessKeyGuard = guard{
	what: "tail-less entity.RelationKey",
	list: "taillessKeyAllowlist",
	advice: "name the tail the edge hangs from: the relation's own Identity() for an edge you hold, " +
		"or the face of the source row you resolved. A key with no FromFace addresses the " +
		"identity edge only, so on a faced source it silently misses the edge the caller meant. " +
		"If you only moved an existing literal between files, move its allowlist entry with it",
}

// taillessKeys returns the position of every entity.RelationKey composite
// literal that names a source but no tail: it sets From and either omits
// FromFace or sets it to an empty string literal. The rules match bareRefs:
// syntactic, any import alias, elided elements of Key collections included,
// and a FromFace variable that happens to be empty is not caught.
func taillessKeys(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	check := func(lit *ast.CompositeLit) {
		face, hasFace := keyValue(lit, "FromFace")
		if _, hasFrom := keyValue(lit, "From"); hasFrom && (!hasFace || isEmptyString(face)) {
			found = append(found, fset.Position(lit.Pos()))
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if isRelationKeyType(lit.Type) {
			check(lit)
			return true
		}
		if elem := collectionElem(lit.Type); elem != nil && isRelationKeyType(elem) {
			for _, elt := range lit.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					elt = kv.Value
				}
				if inner, ok := elt.(*ast.CompositeLit); ok && inner.Type == nil {
					check(inner)
				}
			}
		}
		return true
	})
	return found
}

// collectionElem returns the element type of a slice, array or map type, or
// nil for any other type.
func collectionElem(t ast.Expr) ast.Expr {
	switch t := t.(type) {
	case *ast.ArrayType:
		return t.Elt
	case *ast.MapType:
		return t.Value
	}
	return nil
}

func isRelationKeyType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "RelationKey"
	case *ast.SelectorExpr:
		return t.Sel.Name == "RelationKey"
	}
	return false
}

// TestNoNewTaillessRelationKeys pins every tail-less entity.RelationKey
// literal in non-test code to taillessKeyAllowlist, exactly.
func TestNoNewTaillessRelationKeys(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, scannedRoots, taillessKeys)
	checkAllowlist(t, taillessKeyGuard, got, taillessKeyAllowlist)
}

func TestTaillessKeyAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	checkReasons(t, taillessKeyAllowlist)
}

func TestTaillessKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no tail", `k := entity.RelationKey{From: f, Type: t, To: to}; _ = k`, 1},
		{"aliased package", `k := entityPkg.RelationKey{From: f}; _ = k`, 1},
		{"unqualified", `k := RelationKey{From: f}; _ = k`, 1},
		{"empty tail literal", `k := entity.RelationKey{From: f, FromFace: ""}; _ = k`, 1},
		{"converted empty tail", `k := entity.RelationKey{From: f, FromFace: entity.Face("")}; _ = k`, 1},
		{"named tail", `k := entity.RelationKey{From: f, FromFace: face}; _ = k`, 0},
		{"zero value", `k := entity.RelationKey{}; _ = k`, 0},
		{"own identity", `st.GetRelation(ctx, r.Identity())`, 0},
		{"in a call", `st.GetRelation(ctx, entity.RelationKey{From: f, Type: t, To: to})`, 1},
		{"other type", `r := entity.Ref{ID: id}; _ = r`, 0},
		{"elided in a slice", `ks := []entity.RelationKey{{From: f}, {From: f, FromFace: face}}; _ = ks`, 1},
		{"elided in a map", `m := map[string]entity.RelationKey{"k": {From: f}}; _ = m`, 1},
		{"slice of another type", `rs := []entity.Ref{{ID: id}}; _ = rs`, 0},
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
			if got := taillessKeys(fset, file); len(got) != tc.want {
				t.Errorf("findings = %v, want %d", got, tc.want)
			}
		})
	}
}
