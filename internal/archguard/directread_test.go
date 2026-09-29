package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// directReadRoots are the surfaces that must read single entities through
// visibility.Resolver (design 8.5 of TKT-2528AB): the HTTP API, MCP and Lua.
var directReadRoots = []string{"internal/dataentry", "internal/mcp", "internal/lua"}

// directReadGuard is the 8.5 rule's failure vocabulary.
var directReadGuard = guard{
	what: "direct entity read",
	list: "directReadAllowlist",
	advice: "read a single entity through visibility.Resolver (in dataentry, visibleReader: address, ref, " +
		"inWorld, family), which runs the row gate and the face gate before the load. " +
		"A write-prep read keeps raw access through entityReader.writePrepRow. " +
		"If you only moved an existing read between files, move its allowlist entry with it",
}

// directReads returns the position of every read the 8.5 rule forbids on a
// gated surface:
//
//   - `x.GetEntityState` with three arguments, whatever the face;
//   - `x.GetEntityAt` called or taken as a value, whatever the receiver;
//   - an `EntityQuery{...}` composite literal (bare or qualified) that sets
//     the `IDs` key, the id-batch form of ListEntities / ListEntityHeaders.
//
// The match is syntactic, like zeroFaceReads. Not caught: a query whose IDs
// are assigned after the literal (`q.IDs = ids`), an unkeyed (positional)
// EntityQuery literal, which go vet already rejects for an imported struct,
// and a read hidden behind a helper in another package. Review is the
// backstop for those.
func directReads(fset *token.FileSet, file *ast.File) []token.Position {
	calls := map[ast.Expr]*ast.CallExpr{}
	ast.Inspect(file, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			calls[c.Fun] = c
		}
		return true
	})

	var reads []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			switch n.Sel.Name {
			case "GetEntityState":
				if call := calls[n]; call != nil && len(call.Args) == 3 {
					reads = append(reads, fset.Position(n.Sel.Pos()))
				}
			case "GetEntityAt":
				reads = append(reads, fset.Position(n.Sel.Pos()))
			}
		case *ast.CompositeLit:
			if isEntityQuery(n.Type) && setsKey(n, "IDs") {
				reads = append(reads, fset.Position(n.Pos()))
			}
		}
		return true
	})
	return reads
}

func isEntityQuery(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "EntityQuery"
	case *ast.SelectorExpr:
		return t.Sel.Name == "EntityQuery"
	}
	return false
}

func setsKey(lit *ast.CompositeLit, key string) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
			return true
		}
	}
	return false
}

// TestNoNewDirectReads pins every direct entity read on the gated surfaces
// to directReadAllowlist, exactly. PR 3 and PR 5 of TKT-2528AB shrink it
// until only write-prep entries remain.
func TestNoNewDirectReads(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot, directReadRoots, directReads)
	if len(got) == 0 {
		t.Fatal("scanned no direct reads at all; the walk is probably rooted wrong")
	}
	counts := make(map[string]int, len(directReadAllowlist))
	for path, e := range directReadAllowlist {
		counts[path] = e.n
	}
	for _, msg := range diffAllowlist(directReadGuard, got, counts) {
		t.Error(msg)
	}
}

// Every entry must say why it may read directly: the allowlist is the
// record of which reads are write-prep and which are still to migrate.
func TestDirectReadAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	for path, e := range directReadAllowlist {
		if strings.TrimSpace(e.reason) == "" {
			t.Errorf("%s: allowlist entry has no reason", path)
		}
		if e.n <= 0 {
			t.Errorf("%s: allowlist count %d; delete the entry instead", path, e.n)
		}
	}
}

func TestDirectReads(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"GetEntityState any face", `st.GetEntityState(ctx, id, face)`, 1},
		{"GetEntityState zero face", `st.GetEntityState(ctx, id, "")`, 1},
		{"GetEntityState other arity", `c.GetEntityState(ctx, id)`, 0},
		{"GetEntityAt call", `store.GetEntityAt(ctx, st, addr)`, 1},
		{"GetEntityAt value", `f := store.GetEntityAt; _ = f`, 1},
		{"qualified query with IDs", `st.ListEntities(ctx, store.EntityQuery{IDs: ids})`, 1},
		{"bare query with IDs", `q := EntityQuery{Type: "x", IDs: ids}; _ = q`, 1},
		{"query without IDs", `st.ListEntityHeaders(ctx, store.EntityQuery{Type: "x"})`, 0},
		{"other literal with IDs", `q := RelationQuery{IDs: ids}; _ = q`, 0},
		{"resolver read", `vr.address(ctx, typ, addr)`, 0},
		{"write-prep read", `er.writePrepRow(ctx, ref)`, 0},
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
			if got := directReads(fset, file); len(got) != tc.want {
				t.Errorf("reads = %v, want %d", got, tc.want)
			}
		})
	}
}

// The rule covers the three gated surfaces only: the same read elsewhere is
// not a finding.
func TestScanTree_DirectReadsScopedToGatedSurfaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, src := range map[string]string{
		"internal/mcp/a.go":       "package mcp\nfunc f() { store.GetEntityAt(ctx, st, a) }\n",
		"internal/lua/b.go":       "package lua\nfunc f() { st.GetEntityState(ctx, id, face) }\n",
		"internal/dataentry/c.go": "package dataentry\nvar q = store.EntityQuery{IDs: ids}\n",
		"internal/cli/d.go":       "package cli\nfunc f() { store.GetEntityAt(ctx, st, a) }\n",
		"internal/mcp/a_test.go":  "package mcp\nfunc g() { store.GetEntityAt(ctx, st, a) }\n",
	} {
		if err := mkdirAllWrite(root+"/"+rel, src); err != nil {
			t.Fatal(err)
		}
	}
	got := scanTree(t, root, directReadRoots, directReads)
	if len(got) != 3 || got["internal/cli/d.go"] != nil || got["internal/mcp/a_test.go"] != nil {
		t.Fatalf("scan = %v, want one finding in each gated package and none elsewhere", got)
	}
}
