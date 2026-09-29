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

// scannedRoots are the trees whose non-test Go files a tree-wide guard reads.
var scannedRoots = []string{"internal", "cmd"}

// skippedDirs are directory base names never descended into. testdata holds
// fixtures, not code; node_modules is the SPA's dependency tree.
var skippedDirs = map[string]bool{
	"testdata":     true,
	"node_modules": true,
}

// excludedTrees are repo-relative directories exempt as a whole. The two
// conformance harnesses exercise the store and reader contracts themselves,
// including the zero face of a faceless type, which is exactly what the
// guards steer callers away from. internal/entity defines Ref, so a literal
// there is the type's own construction, not a caller's read.
var excludedTrees = map[string]string{
	"internal/entity":                    "defines entity.Ref",
	"internal/store/storetest":           "store conformance harness; tests GetEntity itself",
	"internal/visibility/visibilitytest": "reader conformance harness; tests GetEntity itself",
}

// allowed is one file's pinned finding count and why it is permitted.
type allowed struct {
	n      int
	reason string
}

// matcher returns the position of every finding a guard makes in one file.
type matcher func(fset *token.FileSet, file *ast.File) []token.Position

// scanTree returns match's findings per repo-relative file under the given
// roots, for every non-test file with at least one.
func scanTree(t *testing.T, root string, roots []string, match matcher) map[string][]token.Position {
	t.Helper()
	got := map[string][]token.Position{}
	fset := token.NewFileSet()
	for _, sub := range roots {
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
			if reads := match(fset, file); len(reads) > 0 {
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

// guard names what a shrink-only allowlist pins, for its failure messages.
type guard struct {
	what   string // the finding, e.g. "direct entity read"
	list   string // the allowlist variable's name
	advice string // what to do instead, appended to every growth message
}

// counts reduces an allowlist to its per-file counts.
func counts(list map[string]allowed) map[string]int {
	out := make(map[string]int, len(list))
	for path, e := range list {
		out[path] = e.n
	}
	return out
}

// checkAllowlist fails t on every file whose findings differ from list.
func checkAllowlist(t *testing.T, g guard, got map[string][]token.Position, list map[string]allowed) {
	t.Helper()
	for _, msg := range diffAllowlist(g, got, counts(list)) {
		t.Error(msg)
	}
}

// checkReasons fails t on an entry with no reason or a count below one: the
// allowlist is the record of why each finding may stay.
func checkReasons(t *testing.T, list map[string]allowed) {
	t.Helper()
	for path, e := range list {
		if strings.TrimSpace(e.reason) == "" {
			t.Errorf("%s: allowlist entry has no reason", path)
		}
		if e.n <= 0 {
			t.Errorf("%s: allowlist count %d; delete the entry instead", path, e.n)
		}
	}
}

// diffAllowlist compares the scanned findings against the allowlist and
// returns one message per file whose count differs, sorted by path. Growth
// messages list the lines of every finding in the file, since a count alone
// does not say which one is new.
func diffAllowlist(g guard, got map[string][]token.Position, allowed map[string]int) []string {
	var msgs []string
	for path, reads := range got {
		n := len(reads)
		want, listed := allowed[path]
		switch {
		case !listed:
			msgs = append(msgs, fmt.Sprintf("%s: %d new %s(s) in a file not on the allowlist (%s); %s",
				path, n, g.what, lines(reads), g.advice))
		case n > want:
			msgs = append(msgs, fmt.Sprintf("%s: %d %ss (%s), allowlist permits %d; %s",
				path, n, g.what, lines(reads), want, g.advice))
		case n < want:
			msgs = append(msgs, fmt.Sprintf("%s: %d %ss, allowlist says %d; "+
				"lower the entry in %s to %d (it may only shrink)", path, n, g.what, want, g.list, n))
		}
	}
	for path, want := range allowed {
		if _, found := got[path]; !found {
			msgs = append(msgs, fmt.Sprintf("%s: no %ss left, allowlist says %d; "+
				"delete the entry from %s", path, g.what, want, g.list))
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

// readsAt builds a scan result of findings at the given lines.
func readsAt(lines ...int) []token.Position {
	out := make([]token.Position, len(lines))
	for i, l := range lines {
		out[i] = token.Position{Line: l}
	}
	return out
}

func TestDiffAllowlist(t *testing.T) {
	t.Parallel()
	g := guard{what: "finding", list: "someAllowlist", advice: "do the other thing"}
	type scan = map[string][]token.Position
	cases := []struct {
		name    string
		got     scan
		allowed map[string]int
		want    []string // substrings, one per expected message
	}{
		{"exact match passes", scan{"a.go": readsAt(3, 9)}, map[string]int{"a.go": 2}, nil},
		{"new file fails", scan{"a.go": readsAt(7)}, map[string]int{}, []string{"a.go: 1 new finding"}},
		{"over count fails", scan{"a.go": readsAt(1, 2, 3)}, map[string]int{"a.go": 2},
			[]string{"(line 1, line 2, line 3), allowlist permits 2"}},
		{"under count fails", scan{"a.go": readsAt(1)}, map[string]int{"a.go": 2}, []string{"lower the entry"}},
		{"gone file fails", scan{}, map[string]int{"a.go": 2}, []string{"delete the entry"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msgs := diffAllowlist(g, tc.got, tc.allowed)
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
	msg := diffAllowlist(g, scan{"a.go": readsAt(7)}, nil)[0]
	for _, want := range []string{"line 7", "do the other thing"} {
		if !strings.Contains(msg, want) {
			t.Errorf("growth message lacks %q: %s", want, msg)
		}
	}
}

func mkdirAllWrite(path, src string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(src), 0o600)
}
