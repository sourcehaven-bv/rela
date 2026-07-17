package attachment

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func newRunner(t *testing.T) *CmdRunner {
	t.Helper()
	r, err := NewCmdRunner(5*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixtures not available on Windows")
	}
}

func TestCmdRunner_NewValidates(t *testing.T) {
	if _, err := NewCmdRunner(0, 1); err == nil {
		t.Error("zero timeout should error")
	}
	if _, err := NewCmdRunner(time.Second, 0); err == nil {
		t.Error("zero maxBytes should error")
	}
}

func TestCmdRunner_Probe(t *testing.T) {
	r := newRunner(t)
	if err := r.Probe([]string{"go"}); err != nil {
		t.Errorf("go should be on PATH: %v", err)
	}
	if err := r.Probe([]string{"definitely-not-a-real-binary-xyz"}); err == nil {
		t.Error("missing binary should fail probe")
	}
	if err := r.Probe(nil); err == nil {
		t.Error("empty command should fail probe")
	}
}

func TestCmdRunner_ScanCleanViaStdin(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	// `true` exits 0 → clean.
	if err := r.Scan(context.Background(), []string{"true"}, []byte("anything")); err != nil {
		t.Errorf("true should be clean: %v", err)
	}
}

func TestCmdRunner_ScanRejectsOnNonZero(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	// `false` exits 1 → rejected.
	err := r.Scan(context.Background(), []string{"false"}, []byte("infected"))
	if !errors.Is(err, ErrRejected) {
		t.Errorf("false should reject (wrap ErrRejected); got %v", err)
	}
}

func TestCmdRunner_ScanFailClosedWhenMissing(t *testing.T) {
	r := newRunner(t)
	// Missing binary → couldn't run → fail closed (rejected).
	err := r.Scan(context.Background(), []string{"no-such-scanner-xyz"}, []byte("x"))
	if !errors.Is(err, ErrRejected) {
		t.Errorf("missing scanner must fail closed (ErrRejected); got %v", err)
	}
}

func TestCmdRunner_TransformViaStdout(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	// `cat` echoes stdin to stdout — identity transform.
	out, name, err := r.Transform(context.Background(), []string{"cat"}, ProcessContext{}, []byte("hello"))
	if err != nil {
		t.Fatalf("cat transform: %v", err)
	}
	if string(out) != "hello" {
		t.Errorf("output = %q, want hello", string(out))
	}
	if name != "" {
		t.Errorf("name = %q, want empty", name)
	}
}

func TestCmdRunner_TransformViaInOutFiles(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not available")
	}
	r := newRunner(t)
	// `cp {in} {out}` round-trips the bytes through runner-owned temp files.
	out, _, err := r.Transform(context.Background(), []string{"cp", templateIn, templateOut}, ProcessContext{}, []byte("filebytes"))
	if err != nil {
		t.Fatalf("cp transform: %v", err)
	}
	if string(out) != "filebytes" {
		t.Errorf("output = %q, want filebytes", string(out))
	}
}

func TestCmdRunner_ArrayArgsNoShellInjection(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	// A shell metacharacter in the DATA must never be interpreted: we scan with
	// `true` and pass nasty bytes; nothing is executed, scan is clean.
	if err := r.Scan(context.Background(), []string{"true"}, []byte("; rm -rf / #")); err != nil {
		t.Errorf("data must be inert: %v", err)
	}
	// And an arg that looks like a shell command is just an arg to `echo` — not
	// a pipeline. echo exits 0.
	out, _, err := r.Transform(context.Background(), []string{"echo", "$(whoami)"}, ProcessContext{}, []byte("x"))
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	if string(out) != "$(whoami)\n" {
		t.Errorf("output = %q, want the literal arg (no shell expansion)", string(out))
	}
}

func TestCmdRunner_Timeout(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	r, err := NewCmdRunner(100*time.Millisecond, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, _, terr := r.Transform(context.Background(), []string{"sleep", "10"}, ProcessContext{}, []byte("x"))
	if terr == nil {
		t.Error("expected timeout error")
	}
}
