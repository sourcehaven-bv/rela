package cmdexec

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func newRunner(t *testing.T) *Runner {
	t.Helper()
	r, err := New(5*time.Second, 1<<20)
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

func TestNew_Validates(t *testing.T) {
	if _, err := New(0, 1); err == nil {
		t.Error("zero timeout should error")
	}
	if _, err := New(time.Second, 0); err == nil {
		t.Error("zero maxBytes should error")
	}
}

func TestProbe(t *testing.T) {
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

func TestRun_Stdout(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	out, usedOut, err := r.Run(context.Background(), []string{"cat"}, []byte("hello"), true)
	if err != nil {
		t.Fatalf("cat: %v", err)
	}
	if usedOut {
		t.Error("cat uses stdout, not {out}")
	}
	if string(out) != "hello" {
		t.Errorf("out = %q, want hello", out)
	}
}

func TestRun_InOutFiles(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not available")
	}
	r := newRunner(t)
	out, usedOut, err := r.Run(context.Background(), []string{"cp", TemplateIn, TemplateOut}, []byte("filebytes"), true)
	if err != nil {
		t.Fatalf("cp: %v", err)
	}
	if !usedOut {
		t.Error("cp uses {out}")
	}
	if string(out) != "filebytes" {
		t.Errorf("out = %q, want filebytes", out)
	}
}

func TestRun_NonZeroExitWrapsExitError(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	_, _, err := r.Run(context.Background(), []string{"false"}, []byte("x"), false)
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("non-zero exit should wrap *exec.ExitError; got %v", err)
	}
}

func TestRun_NoShellInterpretation(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	// A shell-looking arg is a literal arg to echo, never a pipeline.
	out, _, err := r.Run(context.Background(), []string{"echo", "$(whoami)"}, []byte("x"), true)
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	if string(out) != "$(whoami)\n" {
		t.Errorf("out = %q, want literal (no shell expansion)", out)
	}
}

func TestRun_Timeout(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	r, err := New(100*time.Millisecond, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, _, terr := r.Run(context.Background(), []string{"sleep", "10"}, []byte("x"), true)
	if terr == nil {
		t.Error("expected timeout error")
	}
}

func TestRun_OutputCapExceeded(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("head"); err != nil {
		t.Skip("head not available")
	}
	r, err := New(5*time.Second, 8) // tiny cap
	if err != nil {
		t.Fatal(err)
	}
	// `yes` would stream forever; cap must trip and error rather than OOM.
	_, _, rerr := r.Run(context.Background(), []string{"cat"}, []byte("way more than eight bytes of data"), true)
	if rerr == nil {
		t.Error("oversize output should error")
	}
}

func TestRun_EmptyCommand(t *testing.T) {
	r := newRunner(t)
	if _, _, err := r.Run(context.Background(), nil, []byte("x"), true); err == nil {
		t.Error("empty command should error")
	}
}
