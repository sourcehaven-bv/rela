package rewrite

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFile_KeepsLineNumbersAndDirectives(t *testing.T) {
	src := []byte(`package p

import (
	"context"
	_ "embed"
)

//go:embed x.txt
var x string

//go:noinline
func skipped() {}

func (s *S[T]) M(ctx context.Context, n int) error {
	f := func(a int,
		b int) int { return a + b }
	go s.run(ctx)
	go func() {
		_ = f(1, 2)
	}()
	return nil
}

func named() (n int, _ error) { return 1, nil }

func loose(v any, w interface{}, x interface{ M() }) any { return v }

func tagged() {
	_ = func(x struct{ A int "json:\"a,  omitempty\"" }) {}
	go (func() {})()
}
`)
	out, changed, err := File("p.go", src, "internal/p")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected a change")
	}
	got := string(out)
	if a, b := strings.Count(string(src), "\n"), strings.Count(got, "\n"); a != b {
		t.Errorf("line count changed from %d to %d:\n%s", a, b, got)
	}
	for _, want := range []string{
		"//go:embed x.txt\nvar x string",
		"func skipped() {}",
		`seqtrace__rt.Enter(ctx, "internal/p", "(*S).M", "n", n)`,
		"M(ctx context.Context, n int) (seqtrace__r0 error)",
		"defer func() { seqtrace__f.Exit(seqtrace__r0) }();",
		"b int) (seqtrace__r0 int) {",
		"func named() (n int, seqtrace__r1 error) {",
		`"v", seqtrace__rt.Opaque(v), "w", seqtrace__rt.Opaque(w), "x", x)`,
		"Exit(seqtrace__rt.Opaque(seqtrace__r0))",
		`_ = func(x struct{ A int "json:\"a,  omitempty\"" }) {}`,
		`EnterClosure(nil, seqtrace__c, true, "internal/p", "tagged.func2")`,
		"ctx = seqtrace__f.Ctx(ctx);",
		`seqtrace__rt.Spawn("run"); go s.run(ctx)`,
		`seqtrace__rt.EnterClosure(nil, seqtrace__c, false, "internal/p", "(*S).M.func1", "a", a, "b", b)`,
		`seqtrace__rt.EnterClosure(nil, seqtrace__c, true, "internal/p", "(*S).M.func2")`,
		"func() func(a int, b int) int { seqtrace__c := seqtrace__rt.Current(); return func(a int,",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestFile_WrapsYield(t *testing.T) {
	src := []byte(`package p

import "iter"

func Seq() iter.Seq2[int, error] {
	return func(yield func(int,
		error) bool) {
		yield(1, nil)
	}
}

func emit[T any](yield func(v T) bool, v T) { yield(v) }

func notYield(f func(int) bool) {}

func variadic(yield func(...int) bool) {}

func notBool(yield func(int)) {}
`)
	out, _, err := File("p.go", src, "internal/p")
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if a, b := strings.Count(string(src), "\n"), strings.Count(got, "\n"); a != b {
		t.Errorf("line count changed from %d to %d:\n%s", a, b, got)
	}
	for _, want := range []string{
		"{ seqtrace__y := yield; yield = func(seqtrace__y0 int, seqtrace__y1 error) bool { " +
			"defer seqtrace__rt.Suspend(seqtrace__f).Resume(); return seqtrace__y(seqtrace__y0, seqtrace__y1) } };",
		"{ seqtrace__y := yield; yield = func(seqtrace__y0 T) bool { " +
			"defer seqtrace__rt.Suspend(seqtrace__f).Resume(); return seqtrace__y(seqtrace__y0) } };",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "seqtrace__y :="); n != 2 {
		t.Errorf("wrapped %d yields, want 2 (not notYield, variadic or notBool):\n%s", n, got)
	}
}

func TestFile_SkipsCgoAndEmptyFiles(t *testing.T) {
	for name, src := range map[string]string{
		"cgo":   "package p\n\nimport \"C\"\n\nfunc F() {}\n",
		"types": "package p\n\ntype T struct{}\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, changed, err := File("p.go", []byte(src), "p")
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				t.Error("expected no change")
			}
		})
	}
}

// TestDemo instruments testdata/demo (but not its helper package, which
// stands in for library code), runs it and checks how every cross-goroutine
// call found its parent.
func TestDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a program")
	}
	dir := t.TempDir()
	orig, err := filepath.Abs("testdata/demo/main.go")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(orig)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := File(orig, src, "demo")
	if err != nil {
		t.Fatal(err)
	}
	rewritten := filepath.Join(dir, "main.go")
	writeFile(t, rewritten, out)
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{orig: rewritten}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	writeFile(t, overlayPath, overlay)

	trace := filepath.Join(dir, "trace.jsonl")
	cmd := exec.Command("go", "run", "-overlay", overlayPath, "./testdata/demo")
	cmd.Env = append(os.Environ(), "SEQTRACE_OUT="+trace)
	if b, runErr := cmd.CombinedOutput(); runErr != nil {
		t.Fatalf("go run: %v\n%s", runErr, b)
	}

	type ev struct {
		E, V, Fn string
		ID, P    uint64
	}
	f, err := os.Open(trace)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	byID := map[uint64]ev{}
	var calls []ev
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e ev
		if err := json.Unmarshal(bytes.TrimSpace(sc.Bytes()), &e); err != nil {
			t.Fatal(err)
		}
		if e.E == "c" {
			byID[e.ID] = e
			calls = append(calls, e)
		}
	}

	// Each case: a call, and the (parent function, via) it must have.
	want := map[string][2]string{
		"run":       {"main", ""},
		"spawned":   {"run", "go"},
		"run.func1": {"run", "go"},
		"run.func2": {"run", "closure"},
		"worker":    {"run", "go"},
		"handle":    {"run", "ctx"},
		// Loop bodies over iterators belong to the function with the loop.
		"bodySimple":  {"iterate", ""},
		"bodyHelper":  {"iterate", ""},     // the iterator hands yield to emit
		"bodyAdapter": {"iterate", ""},     // adapt yields from its own loop
		"adapterBody": {"adapt.func1", ""}, // adapt's own loop body over nums
		"afterBreak":  {"iterate", ""},
		// After an early break, the iterator's own calls stay under it.
		"producerTail": {"nums.func1", ""},
	}
	seen := map[string]bool{}
	for _, c := range calls {
		w, ok := want[c.Fn]
		if !ok {
			continue
		}
		seen[c.Fn] = true
		if got := byID[c.P].Fn; got != w[0] || c.V != w[1] {
			t.Errorf("%s: parent %q via %q, want %q via %q", c.Fn, got, c.V, w[0], w[1])
		}
	}
	for fn := range want {
		if !seen[fn] {
			t.Errorf("no call recorded for %s", fn)
		}
	}
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}
