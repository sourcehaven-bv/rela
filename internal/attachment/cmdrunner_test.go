package attachment

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
)

// templateIn / templateOut mirror the cmdexec placeholders for test commands.
const (
	templateIn  = cmdexec.TemplateIn
	templateOut = cmdexec.TemplateOut
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

// TestCmdRunnerBindsScannerDefaults pins the WIRING, not the constant. The
// defaults being well-formed says nothing about anything reading them: before
// this test, deleting the DefaultScannerConfigs append from NewCmdRunner
// reintroduced the bug this package exists to prevent — clamdscan unable to
// parse /etc/clamav/clamd.conf, so every upload rejected — while leaving
// internal/attachment, internal/cmdexec and internal/metamodel all green.
//
// Both lists are asserted: the socket is how the scanner is reached, the config
// is how it learns where the socket is, and having only one looks like having
// neither.
func TestCmdRunnerBindsScannerDefaults(t *testing.T) {
	r, err := NewCmdRunner(time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	bound := map[string]bool{}
	for _, p := range r.exec.ExtraReadOnly() {
		bound[p] = true
	}
	for _, want := range cmdexec.DefaultScannerSockets {
		if !bound[want] {
			t.Errorf("scanner socket %q is not bound; the scanner is unreachable", want)
		}
	}
	for _, want := range cmdexec.DefaultScannerConfigs {
		if !bound[want] {
			t.Errorf("scanner config %q is not bound; clamdscan cannot find LocalSocket", want)
		}
	}
}

// TestCmdRunnerBindsOperatorPathsAlongsideDefaults pins that an operator's
// scan_sockets entries are added to the defaults rather than replacing them —
// supplying one custom path must not silently unbind the stock locations.
func TestCmdRunnerBindsOperatorPathsAlongsideDefaults(t *testing.T) {
	const custom = "/opt/clamav/run/clamd.sock"
	r, err := NewCmdRunner(time.Second, 1<<20, WithScannerSockets(custom))
	if err != nil {
		t.Fatal(err)
	}
	var sawCustom, sawDefault bool
	for _, p := range r.exec.ExtraReadOnly() {
		switch p {
		case custom:
			sawCustom = true
		case cmdexec.DefaultScannerConfigs[0]:
			sawDefault = true
		}
	}
	if !sawCustom {
		t.Errorf("operator path %q not bound", custom)
	}
	if !sawDefault {
		t.Errorf("operator path replaced the defaults instead of extending them")
	}
}
