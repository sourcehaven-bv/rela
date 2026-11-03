package cmdexec

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func TestRunWith_InputName(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	res, err := r.RunWith(context.Background(),
		[]string{"sh", "-c", `basename "$(dirname "$1")"; basename "$1"; cat "$1"`, "sh", TemplateIn}, []byte("body"),
		RunSpec{InputName: "procedure/PROC-1.md"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Output) != "procedure\nPROC-1.md\nbody" {
		t.Errorf("out = %q", res.Output)
	}
}

func TestRunWith_InvalidInputName(t *testing.T) {
	r := newRunner(t)
	for _, name := range []string{
		"../x.md", "a/../b.md", "a/b/c.md", "/abs.md", "a/", "a//b.md", ".vale.ini", "a/.x",
		"out", "OUT", "out/x.md", "Out/x.md", "a b.md", "x*.md", "a..md",
	} {
		if _, err := r.RunWith(context.Background(), []string{"cat", TemplateIn}, nil, RunSpec{InputName: name}); err == nil {
			t.Errorf("input name %q accepted", name)
		}
	}
}

func TestRunWith_AcceptedExitCode(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	cmd := []string{"sh", "-c", "echo findings; exit 1"}

	res, err := r.RunWith(context.Background(), cmd, nil, RunSpec{AcceptedExitCodes: []int{0, 1}})
	if err != nil {
		t.Fatalf("accepted exit 1: %v", err)
	}
	if res.ExitCode != 1 || string(res.Output) != "findings\n" {
		t.Errorf("res = %+v", res)
	}

	_, err = r.RunWith(context.Background(), cmd, nil, RunSpec{})
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("unaccepted exit should wrap *exec.ExitError; got %v", err)
	}

	_, err = r.RunWith(context.Background(), []string{"sh", "-c", "exit 2"}, nil, RunSpec{AcceptedExitCodes: []int{1}})
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Errorf("exit 2 with only 1 accepted: %v", err)
	}
}

func TestRunWith_TimeoutOnlyShortens(t *testing.T) {
	skipOnWindows(t)
	r, err := New(200*time.Millisecond, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.RunWith(context.Background(), []string{"sleep", "10"}, nil, RunSpec{Timeout: 50 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "timed out after 50ms") {
		t.Errorf("err = %v, want a 50ms timeout", err)
	}
	_, err = r.RunWith(context.Background(), []string{"sleep", "10"}, nil, RunSpec{Timeout: time.Hour})
	if err == nil || !strings.Contains(err.Error(), "timed out after 200ms") {
		t.Errorf("err = %v, want the runner's 200ms bound", err)
	}
}

func TestRunWith_WorkingDirIsTempDir(t *testing.T) {
	skipOnWindows(t)
	r := newRunner(t)
	res, err := r.RunWith(context.Background(), []string{"sh", "-c", `pwd -P`}, nil, RunSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.Base(strings.TrimSpace(string(res.Output))), "rela-cmdexec-") {
		t.Errorf("working dir = %q, want the run's temp dir", res.Output)
	}
}

func TestRunWith_OutputCapAppliesToAcceptedExit(t *testing.T) {
	skipOnWindows(t)
	r, err := New(5*time.Second, 8)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.RunWith(context.Background(), []string{"sh", "-c", "echo way more than eight bytes; exit 1"}, nil,
		RunSpec{AcceptedExitCodes: []int{1}})
	if err == nil {
		t.Error("oversize output with an accepted exit code should error")
	}
}
