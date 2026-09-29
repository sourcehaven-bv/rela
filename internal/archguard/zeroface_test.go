// Package archguard holds tree-wide structural tests: guards that scan the
// source of every package rather than exercising one. It has no non-test
// files, so nothing imports it and it adds no edge to the package graph.
package archguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repoRoot is the module root relative to this package's directory, which is
// the working directory `go test` runs the test in.
const repoRoot = "../.."

// scannedRoots are the trees whose non-test Go files the guard reads.
var scannedRoots = []string{"internal", "cmd"}

// skippedDirs are directory base names never descended into. testdata holds
// fixtures, not code; node_modules is the SPA's dependency tree.
var skippedDirs = map[string]bool{
	"testdata":     true,
	"node_modules": true,
}

// excludedTrees are repo-relative directories exempt as a whole. Both are
// conformance harnesses: their zero-face calls test the store's own
// GetEntity contract (the thing this guard steers callers away from), so
// they change with that API in stage 2 of RES-Y6JA37 rather than shrinking
// call by call.
var excludedTrees = map[string]string{
	"internal/store/storetest":           "store conformance harness; tests GetEntity itself",
	"internal/visibility/visibilitytest": "reader conformance harness; tests GetEntity itself",
}

// zeroFaceReads returns the position of every zero-face read in a file.
//
// The match is syntactic: the guard has no type information, because
// golang.org/x/tools/go/packages is not a dependency and type-checking the
// whole tree from a unit test would also skip build-tagged backend files. So
// it matches by name and arity:
//
//   - `x.GetEntity` with two arguments, or as a method value. This matches a
//     store.Store receiver and also every consumer-side interface and reader
//     with that shape, including address-taking ones such as
//     visibility.ScriptReader. That over-count is deliberate: the allowlist
//     only shrinks, and moving a call onto an address-aware API removes it.
//     The three-argument cli/sync client method is excluded by arity.
//   - `x.GetEntityState(ctx, id, "")`: the same read with the zero face
//     spelled as a literal, bare or converted (`entity.Face("")`).
//   - `x.getEntity` with two arguments, or as a method value
//     (dataentry.entityReader). The three-argument feed source method is
//     excluded by arity.
//   - any identifier named `bareEntityID` other than its declaration.
//
// Not caught: a read hidden behind a new wrapper function, the zero face
// passed through a variable or named constant, and store.GetEntityAt given
// a bare id (it resolves to the zero face; internal/cli/show.go does this).
// Review is the backstop for those.
func zeroFaceReads(fset *token.FileSet, file *ast.File) []token.Position {
	calls := map[ast.Expr]*ast.CallExpr{}
	decls := map[*ast.Ident]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			calls[n.Fun] = n
		case *ast.FuncDecl:
			decls[n.Name] = true
		}
		return true
	})

	var reads []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			call := calls[n]
			switch n.Sel.Name {
			case "GetEntity", "getEntity":
				if call == nil || len(call.Args) == 2 {
					reads = append(reads, fset.Position(n.Sel.Pos()))
				}
			case "GetEntityState":
				if call != nil && len(call.Args) == 3 && isEmptyFace(call.Args[2]) {
					reads = append(reads, fset.Position(n.Sel.Pos()))
				}
			}
		case *ast.Ident:
			if n.Name == "bareEntityID" && !decls[n] {
				reads = append(reads, fset.Position(n.Pos()))
			}
		}
		return true
	})
	return reads
}

// isEmptyFace reports whether e is an empty string literal, bare or wrapped
// in a one-argument conversion such as entity.Face("").
func isEmptyFace(e ast.Expr) bool {
	if conv, ok := e.(*ast.CallExpr); ok && len(conv.Args) == 1 {
		e = conv.Args[0]
	}
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && (lit.Value == `""` || lit.Value == "``")
}

// scanTree returns the zero-face reads per repo-relative file, for every
// file with at least one.
func scanTree(t *testing.T, root string) map[string][]token.Position {
	t.Helper()
	got := map[string][]token.Position{}
	fset := token.NewFileSet()
	for _, sub := range scannedRoots {
		err := filepath.WalkDir(filepath.Join(root, sub), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				// The go tool ignores directories starting with "." or "_".
				if skippedDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_") {
					return filepath.SkipDir
				}
				if _, ok := excludedTrees[rel]; ok {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if perr != nil {
				return perr
			}
			if reads := zeroFaceReads(fset, file); len(reads) > 0 {
				got[rel] = reads
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", sub, err)
		}
	}
	return got
}

// alternatives is the advice every finding carries.
const alternatives = "read an explicit address instead: store.GetEntityState(ctx, id, face), " +
	"store.GetEntityAt(ctx, r, addr) with an ID@face address, visibleReader.getVisibleRef, " +
	"entityReader.getEntityRef, or a visibility reader (ScriptReader, UnrestrictedReader). " +
	"A faced type stores no zero-face row, so a bare-id read of it finds nothing (DEC-NPZICR). " +
	"If you only moved an existing read between files, move its allowlist count with it"

// diffAllowlist compares the scanned reads against the allowlist and returns
// one message per file whose count differs, sorted by path. Growth messages
// list the lines of every read in the file, since a count alone does not say
// which one is new.
func diffAllowlist(got map[string][]token.Position, allowed map[string]int) []string {
	var msgs []string
	for path, reads := range got {
		n := len(reads)
		want, listed := allowed[path]
		switch {
		case !listed:
			msgs = append(msgs, fmt.Sprintf("%s: %d new zero-face read(s) in a file not on the allowlist (%s); %s",
				path, n, lines(reads), alternatives))
		case n > want:
			msgs = append(msgs, fmt.Sprintf("%s: %d zero-face reads (%s), allowlist permits %d; %s",
				path, n, lines(reads), want, alternatives))
		case n < want:
			msgs = append(msgs, fmt.Sprintf("%s: %d zero-face reads, allowlist says %d; "+
				"lower the entry in zeroFaceAllowlist to %d (it may only shrink)", path, n, want, n))
		}
	}
	for path, want := range allowed {
		if _, found := got[path]; !found {
			msgs = append(msgs, fmt.Sprintf("%s: no zero-face reads left, allowlist says %d; "+
				"delete the entry from zeroFaceAllowlist", path, want))
		}
	}
	slices.Sort(msgs)
	return msgs
}

func lines(reads []token.Position) string {
	parts := make([]string, len(reads))
	for i, p := range reads {
		parts[i] = fmt.Sprintf("line %d", p.Line)
	}
	return strings.Join(parts, ", ")
}

// TestNoNewZeroFaceReads pins every zero-face read in the tree to
// zeroFaceAllowlist, exactly. Stage 0 of RES-Y6JA37: later stages remove the
// listed reads; this stops new ones arriving meanwhile.
func TestNoNewZeroFaceReads(t *testing.T) {
	t.Parallel()
	got := scanTree(t, repoRoot)
	if len(got) == 0 {
		t.Fatal("scanned no zero-face reads at all; the walk is probably rooted wrong")
	}
	for _, msg := range diffAllowlist(got, zeroFaceAllowlist) {
		t.Error(msg)
	}
}

func TestZeroFaceReads(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"store GetEntity call", `st.GetEntity(ctx, id)`, 1},
		{"chained receiver", `a.b.store.GetEntity(ctx, "X-1")`, 1},
		{"method value", `f := st.GetEntity; _ = f`, 1},
		{"sync client arity", `c.GetEntity(ctx, "tickets", id)`, 0},
		{"GetEntityState literal zero face", `st.GetEntityState(ctx, id, "")`, 1},
		{"GetEntityState raw-string zero face", "st.GetEntityState(ctx, id, ``)", 1},
		{"GetEntityState named face", `st.GetEntityState(ctx, id, face)`, 0},
		{"GetEntityState literal face", `st.GetEntityState(ctx, id, "published")`, 0},
		{"GetEntityState converted zero face", `st.GetEntityState(ctx, id, entity.Face(""))`, 1},
		{"GetEntityState converted face", `st.GetEntityState(ctx, id, entity.Face("draft"))`, 0},
		{"entityReader getEntity", `er.getEntity(ctx, id)`, 1},
		{"feed source getEntity arity", `s.getEntity(ctx, typ, id)`, 0},
		{"bareEntityID call", `id, _ := bareEntityID(raw); _ = id`, 1},
		{"bareEntityID value", `f := bareEntityID; _ = f`, 1},
		{"getEntityRef is address-aware", `er.getEntityRef(ctx, ref)`, 0},
		{"GetEntityAt is address-aware", `store.GetEntityAt(ctx, st, addr)`, 0},
		{"two reads", `st.GetEntity(ctx, a); st.GetEntity(ctx, b)`, 2},
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
			if got := zeroFaceReads(fset, file); len(got) != tc.want {
				t.Errorf("reads = %v, want %d", got, tc.want)
			}
		})
	}
}

// Declarations are not reads: an interface method, a method implementation
// and the bareEntityID declaration itself.
func TestZeroFaceReads_IgnoresDeclarations(t *testing.T) {
	t.Parallel()
	src := `package p
type reader interface { GetEntity(ctx context.Context, id string) (*Entity, error) }
func (s *S) GetEntity(ctx context.Context, id string) (*Entity, error) { return nil, nil }
func bareEntityID(raw string) (string, bool) { return raw, true }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := zeroFaceReads(fset, file); len(got) != 0 {
		t.Errorf("declarations counted as reads: %v", got)
	}
}

// readsAt builds a scan result of reads at the given lines.
func readsAt(lines ...int) []token.Position {
	out := make([]token.Position, len(lines))
	for i, l := range lines {
		out[i] = token.Position{Line: l}
	}
	return out
}

func TestDiffAllowlist(t *testing.T) {
	t.Parallel()
	type scan = map[string][]token.Position
	cases := []struct {
		name    string
		got     scan
		allowed map[string]int
		want    []string // substrings, one per expected message
	}{
		{"exact match passes", scan{"a.go": readsAt(3, 9)}, map[string]int{"a.go": 2}, nil},
		{"new file fails", scan{"a.go": readsAt(7)}, map[string]int{}, []string{"a.go: 1 new zero-face read"}},
		{"over count fails", scan{"a.go": readsAt(1, 2, 3)}, map[string]int{"a.go": 2},
			[]string{"(line 1, line 2, line 3), allowlist permits 2"}},
		{"under count fails", scan{"a.go": readsAt(1)}, map[string]int{"a.go": 2}, []string{"lower the entry"}},
		{"gone file fails", scan{}, map[string]int{"a.go": 2}, []string{"delete the entry"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msgs := diffAllowlist(tc.got, tc.allowed)
			if len(msgs) != len(tc.want) {
				t.Fatalf("got %d messages %q, want %d", len(msgs), msgs, len(tc.want))
			}
			for i, sub := range tc.want {
				if !strings.Contains(msgs[i], sub) {
					t.Errorf("message %q lacks %q", msgs[i], sub)
				}
			}
		})
	}
	// Growth must point at the fix, not at the allowlist.
	msg := diffAllowlist(scan{"a.go": readsAt(7)}, nil)[0]
	for _, want := range []string{"line 7", "DEC-NPZICR", "GetEntityState", "getVisibleRef", "getEntityRef"} {
		if !strings.Contains(msg, want) {
			t.Errorf("growth message lacks %q: %s", want, msg)
		}
	}
}

// The guard must fail on a synthetic violation end to end: a tree with an
// unlisted file holding a read yields a finding.
func TestScanTree_FindsSyntheticViolation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := mkdirAllWrite(p, src); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/pkg/bad.go", "package pkg\nfunc f() { st.GetEntity(ctx, id) }\n")
	write("internal/pkg/bad_test.go", "package pkg\nfunc g() { st.GetEntity(ctx, id) }\n")
	write("internal/pkg/testdata/x.go", "package x\nfunc g() { st.GetEntity(ctx, id) }\n")
	write("internal/store/storetest/h.go", "package storetest\nfunc g() { st.GetEntity(ctx, id) }\n")
	write("internal/_scratch/y.go", "package y\nfunc g() { st.GetEntity(ctx, id) }\n")
	write("cmd/tool/main.go", "package main\nfunc main() { st.GetEntity(ctx, id) }\n")

	got := scanTree(t, root)
	if len(got) != 2 || len(got["internal/pkg/bad.go"]) != 1 || len(got["cmd/tool/main.go"]) != 1 {
		t.Fatalf("scan = %v, want one read in each of internal/pkg/bad.go and cmd/tool/main.go", got)
	}
	if msgs := diffAllowlist(got, map[string]int{"cmd/tool/main.go": 1}); len(msgs) != 1 ||
		!strings.HasPrefix(msgs[0], "internal/pkg/bad.go:") {

		t.Errorf("want one finding for the unlisted file; got %q", msgs)
	}
}

func mkdirAllWrite(path, src string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(src), 0o600)
}
