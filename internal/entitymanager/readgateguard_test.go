package entitymanager_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// readGates are the calls that refuse a row the caller cannot read as a
// missing one. A write to an existing row runs one of them before it asks
// the ACL, so a 403 never confirms the row exists (see the entitymanager
// CLAUDE.md, "PatchEntity ordering is load-bearing").
var readGates = map[string]bool{
	"requireReadableRow": true,
	"visibleEndpoint":    true,
	"lookupSource":       true,
}

// noReadGateNeeded lists the functions that call authorizeAndAudit with no
// read gate before it in the same function, each with the reason. Most are
// helpers whose caller gates the row first; the reason names that caller so
// a reviewer can check it. Adding a name is a deliberate exception.
var noReadGateNeeded = map[string]string{
	"CreateEntity":   "creates a row; no stored row exists for a 403 to confirm",
	"recreateEntity": "authorizes a create of a face that is not stored, as CreateEntity does",
	"authorizeTag":   "VersionTags.prepare runs requireReadableRow on the row before calling it",
	"authorizeRename": "RenameEntity runs requireReadableRow on the family before calling it, and " +
		"familyRename.inTx calls it on that family and its renamed copy",
	"authorizeFamily": "DeleteEntity and SoftDeleteEntity gate the family first; owned deletes " +
		"(prepareOwnedDeletes) and restores (authorizeRestore) reach rows through a gated owner or marker",
	"authorizeCascadeRelations": "authorizes the edges a gated delete destroys; the deleted row was " +
		"gated by the delete's entry point",
	"authorizeCopy": "gates the source through CopyReadGate.PermitsReadFace, and planCopy runs " +
		"probeCopyTarget, which read-gates the target, first",
	"edgeTypeAllowed": "an affordance probe for the copy's own edges: withAffordanceProbe drops the " +
		"audit row and the copy skips the edge instead of answering 403",
	"authorizeIncomingSiblings": "authorizes other sources' edges into a target whose own source " +
		"UpdateRelation gated through lookupSource; the sibling sources are not read-gated",
}

// TestAuthorizeAndAuditFollowsReadGate fails when a function in this
// package calls authorizeAndAudit without first calling a read gate, unless
// the function is in noReadGateNeeded. A new write path that asks the ACL
// first would answer 403 for a hidden row and 404 for a missing one.
func TestAuthorizeAndAuditFollowsReadGate(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var bad []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range ungatedAuthorizations(fset, f) {
			seen[v.fn] = true
			if _, ok := noReadGateNeeded[v.fn]; !ok {
				bad = append(bad, v.String())
			}
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("%s: authorizeAndAudit with no read gate before it; run requireReadableRow "+
			"(or visibleEndpoint, lookupSource) first, or add the function to noReadGateNeeded with a reason", b)
	}
	for fn := range noReadGateNeeded {
		if !seen[fn] {
			t.Errorf("noReadGateNeeded lists %s, which no longer calls authorizeAndAudit ungated; remove it", fn)
		}
	}
}

// TestAuthorizeAndAuditFollowsReadGate_Planted pins that the scan catches a
// violation and accepts a gated call, so the guard cannot pass vacuously.
func TestAuthorizeAndAuditFollowsReadGate_Planted(t *testing.T) {
	t.Parallel()
	const src = `package entitymanager

func (m *Manager) Gated(ctx context.Context, row *entity.Entity) error {
	if err := requireReadableRow(ctx, faceGateOf(m), row.ID, nil); err != nil {
		return err
	}
	return m.authorizeAndAudit(ctx, req)
}

func (m *Manager) Planted(ctx context.Context) error {
	if err := m.authorizeAndAudit(ctx, req); err != nil {
		return err
	}
	return requireReadableRow(ctx, faceGateOf(m), "X", nil)
}

func (m *Manager) Closure(ctx context.Context) error {
	return m.store.Tx(ctx, func(ctx context.Context) error {
		return m.authorizeAndAudit(ctx, req)
	})
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "planted.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range ungatedAuthorizations(fset, f) {
		got = append(got, v.fn)
	}
	if want := "Planted,Closure"; strings.Join(got, ",") != want {
		t.Errorf("ungated = %v, want %s: the gate after the call must not count, and a call "+
			"inside a closure belongs to its enclosing function", got, want)
	}
}

type ungated struct {
	fn  string
	pos token.Position
}

func (u ungated) String() string { return u.pos.String() + " (" + u.fn + ")" }

// ungatedAuthorizations returns each authorizeAndAudit call in f that no
// read-gate call precedes, in source order, within the same top-level
// function. Closures count as part of the function that contains them.
func ungatedAuthorizations(fset *token.FileSet, f *ast.File) []ungated {
	var out []ungated
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		firstGate := token.NoPos
		var calls []token.Pos
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch name := calleeName(call); {
			case readGates[name]:
				if !firstGate.IsValid() || call.Pos() < firstGate {
					firstGate = call.Pos()
				}
			case name == "authorizeAndAudit":
				calls = append(calls, call.Pos())
			}
			return true
		})
		for _, pos := range calls {
			if !firstGate.IsValid() || pos < firstGate {
				out = append(out, ungated{fn: fn.Name.Name, pos: fset.Position(pos)})
			}
		}
	}
	return out
}

func calleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}
