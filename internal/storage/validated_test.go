package storage

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatedPath_ZeroValueIsEmpty(t *testing.T) {
	var v ValidatedPath
	if v.String() != "" {
		t.Fatalf("zero ValidatedPath = %q, want empty", v.String())
	}
}

func TestValidatedFS_UnwrapsToUnderlyingFS(t *testing.T) {
	dir := t.TempDir()
	rfs, err := NewRootedFS(NewOsFS(), dir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	full, err := rfs.resolve("a/b.txt")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if mkErr := rfs.vfs.MkdirAll(rfs.parent(full), 0o755); mkErr != nil {
		t.Fatalf("MkdirAll: %v", mkErr)
	}
	if wErr := rfs.vfs.WriteFile(full, []byte("hi"), 0o644); wErr != nil {
		t.Fatalf("WriteFile: %v", wErr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a", "b.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("content = %q, want %q", got, "hi")
	}
}

// TestParent_StaysInsideRoot pins the claim in parent's doc comment: the
// directory of a validated path is itself under the root, so widening the
// invariant to it is sound. A key like "a/b.txt" must not have "." or the
// root's own parent as its parent.
func TestParent_StaysInsideRoot(t *testing.T) {
	dir := t.TempDir()
	rfs, err := NewRootedFS(NewOsFS(), dir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	for _, key := range []string{"a.txt", "a/b.txt", "a/b/c/d.txt"} {
		t.Run(key, func(t *testing.T) {
			full, err := rfs.resolve(key)
			if err != nil {
				t.Fatalf("resolve(%q): %v", key, err)
			}
			p := rfs.parent(full).String()
			if p != rfs.root && !strings.HasPrefix(p, rfs.root+string(filepath.Separator)) {
				t.Fatalf("parent(%q) = %q, outside root %q", key, p, rfs.root)
			}
		})
	}
}

// TestContain_RootIsFilesystemRoot pins the "/" case. Building the prefix as
// root+separator yields "//", which no path has, so every key under a root of
// "/" reads as an escape — a container or a tmpdir at the filesystem root hits
// this, and the failure is a blanket refusal rather than anything subtle.
func TestContain_RootIsFilesystemRoot(t *testing.T) {
	rfs := &RootedFS{root: string(filepath.Separator)}
	for _, key := range []string{"entities", "entities/tickets/a.md"} {
		t.Run(key, func(t *testing.T) {
			got, err := rfs.contain(filepath.Join(rfs.root, key))
			if err != nil {
				t.Fatalf("contain(%q) under root %q: %v", key, rfs.root, err)
			}
			if want := filepath.Join(rfs.root, key); got.String() != want {
				t.Fatalf("contain = %q, want %q", got.String(), want)
			}
		})
	}
}

// TestContain_RejectsEscape pins the postcondition itself: contain is the last
// barrier, so it must refuse a path outside the root even though resolve's
// segment rules should already have made that impossible.
func TestContain_RejectsEscape(t *testing.T) {
	dir := t.TempDir()
	rfs, err := NewRootedFS(NewOsFS(), dir)
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	for _, p := range []string{
		filepath.Join(dir, "..", "outside.md"),
		filepath.Dir(dir),
		dir + "-sibling/x.md", // prefix-of-root without a separator boundary
	} {
		t.Run(p, func(t *testing.T) {
			if _, err := rfs.contain(p); err == nil {
				t.Errorf("contain(%q) = nil error, want rejection (root %q)", p, dir)
			}
		})
	}
}

// TestParent_FallsBackToRootOnEscape drives parent's defensive branch. It is
// unreachable through resolve (a contained path's parent is contained), so the
// only way to make contain fail is a hand-built RootedFS whose root does not
// contain the path. The fallback must be the root itself, never the escaping
// directory.
func TestParent_FallsBackToRootOnEscape(t *testing.T) {
	sep := string(filepath.Separator)
	rfs := &RootedFS{root: filepath.Join(sep, "a")}
	got := rfs.parent(ValidatedPath{p: filepath.Join(sep, "b", "c")})
	if got.String() != rfs.root {
		t.Fatalf("parent of out-of-root path = %q, want the root %q", got.String(), rfs.root)
	}
}

// rootedGoExemptFuncs are the rooted.go functions allowed to read the raw
// `fs` field. Exemption list, not inclusion list, so a new method fails closed.
var rootedGoExemptFuncs = map[string]string{
	"SupportsStreaming": "type-sniffs the backend chain; performs no I/O",
}

// TestRootedGo_NoRawFSOrDirectOSAccess is a guard test in the style of
// internal/acl's ceilingguard_test. It parses rooted.go and fails on either
// shape that bypasses the ValidatedPath barrier:
//
//   - any read of the raw `fs` field outside the exempt functions — Go cannot
//     say "this field may only be read by that method", and a local alias
//     (`raw := r.fs; raw.ReadFile(bareString)`) is zero validation;
//   - any direct os.* call — validated.go is the one place a ValidatedPath is
//     unwrapped, and it exists so that this file never has to.
//
// It walks the AST rather than matching source text: a regex over `r.fs.X`
// is defeated by exactly that alias, which SupportsStreaming already uses,
// and a guard that passes against the regression it names is worse than none.
//
// Scope is rooted.go only, by design. safefs.go and osfs.go ARE the raw layer
// beneath the barrier and are expected to call os.* directly.
func TestRootedGo_NoRawFSOrDirectOSAccess(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "rooted.go", nil, 0)
	if err != nil {
		t.Fatalf("parse rooted.go: %v", err)
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if _, exempt := rootedGoExemptFuncs[fn.Name.Name]; exempt {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if x.Sel.Name == "fs" {
					t.Errorf("%s: %s reads the raw FS field; go through r.vfs so the path carries ValidatedPath",
						fset.Position(x.Pos()), fn.Name.Name)
				}
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" {
						t.Errorf("%s: %s calls os.%s directly, bypassing validatedFS; put the unwrap in validated.go",
							fset.Position(x.Pos()), fn.Name.Name, sel.Sel.Name)
					}
				}
			}
			return true
		})
	}
}
