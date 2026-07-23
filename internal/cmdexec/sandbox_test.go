package cmdexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// requireWorkingSandbox skips when this host has no usable sandbox, so the suite
// stays green on machines without bwrap while still really exercising the
// mechanism where one exists.
func requireWorkingSandbox(t *testing.T) Sandbox {
	t.Helper()
	sb := NewSandbox()
	if err := sb.Available(); err != nil {
		t.Skipf("no working sandbox on this host: %v", err)
	}
	return sb
}

// runWrapped executes argv under the sandbox and returns combined output.
func runWrapped(t *testing.T, sb Sandbox, dir string, argv ...string) (string, error) {
	t.Helper()
	wrapped, err := sb.Wrap(argv, Spec{WritableDir: dir})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, wrapped[0], wrapped[1:]...).CombinedOutput()
	return string(out), err
}

// TestSandboxBlocksEgress is THE test that matters. A sandbox whose profile
// silently fails to apply looks identical to a working one — the command
// succeeds either way — so we assert the negative directly: a process inside the
// sandbox must NOT be able to open an outbound connection.
//
// It connects to a TCP listener this test owns on loopback. Loopback is the
// strongest available check: it needs no internet, and a sandbox that cannot
// even block loopback certainly cannot block the cloud metadata endpoint.
func TestSandboxBlocksEgress(t *testing.T) {
	sb := requireWorkingSandbox(t)
	python := pythonPath(t)

	// A real listener, so "connection refused" cannot be mistaken for "blocked".
	ln := mustListen(t)
	defer ln.Close()
	go acceptLoop(ln)

	dir := t.TempDir()
	script := "import socket,sys\n" +
		"s=socket.socket(); s.settimeout(5)\n" +
		"try:\n" +
		"    s.connect(('127.0.0.1'," + ln.port + "))\n" +
		"    print('CONNECTED')\n" +
		"except Exception as e:\n" +
		"    print('BLOCKED', type(e).__name__)\n"

	// Control: unsandboxed, the connection must succeed — otherwise the test is
	// vacuous (e.g. listener not actually up) and a "blocked" result meaningless.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctrl, err := exec.CommandContext(ctx, python, "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("control run failed: %v: %s", err, ctrl)
	}
	if !strings.Contains(string(ctrl), "CONNECTED") {
		t.Fatalf("control did not connect; test would be vacuous. output=%q", ctrl)
	}

	// Sandboxed: the same connect must be refused by the sandbox.
	out, _ := runWrapped(t, sb, dir, python, "-c", script)
	if strings.Contains(out, "CONNECTED") {
		t.Errorf("SANDBOX DID NOT BLOCK EGRESS — a converter could reach internal "+
			"services or cloud metadata from the server. output=%q", out)
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Errorf("expected the connect to be blocked; output=%q", out)
	}
}

// TestSandboxBlocksWriteOutsideWritableDir pins the filesystem half: the command
// may write its own temp dir and nothing else.
func TestSandboxBlocksWriteOutsideWritableDir(t *testing.T) {
	sb := requireWorkingSandbox(t)
	python := pythonPath(t)
	dir := t.TempDir()

	// Writing INSIDE the writable dir must succeed — otherwise the sandbox is
	// useless for its actual job (producing {out}).
	inside := filepath.Join(dir, "ok.txt")
	out, err := runWrapped(t, sb, dir, python, "-c",
		"open("+pyStr(inside)+",'w').write('x'); print('WROTE')")
	if err != nil || !strings.Contains(out, "WROTE") {
		t.Fatalf("write inside writable dir should succeed: err=%v out=%q", err, out)
	}

	// Writing OUTSIDE must fail. Target a path in the user's home, which the
	// converter has no business touching.
	home, herr := os.UserHomeDir()
	if herr != nil {
		t.Skipf("no home dir: %v", herr)
	}
	outside := filepath.Join(home, ".rela-sandbox-escape-probe")
	defer os.Remove(outside)
	out, _ = runWrapped(t, sb, dir, python, "-c",
		"\ntry:\n    open("+pyStr(outside)+",'w').write('x'); print('WROTE')\n"+
			"except Exception as e:\n    print('DENIED', type(e).__name__)\n")
	if strings.Contains(out, "WROTE") {
		t.Errorf("SANDBOX DID NOT BLOCK WRITE outside the writable dir: %q", out)
	}
	if _, statErr := os.Stat(outside); statErr == nil {
		t.Errorf("file was created outside the sandbox at %s", outside)
	}
}

// TestSandboxWrapDoesNotMutateInput guards a subtle aliasing bug: Wrap must not
// modify the caller's argv slice.
func TestSandboxWrapDoesNotMutateInput(t *testing.T) {
	sb := NewSandbox()
	if err := sb.Available(); err != nil {
		t.Skipf("no sandbox: %v", err)
	}
	argv := []string{"echo", "hello"}
	orig := append([]string(nil), argv...)
	if _, err := sb.Wrap(argv, Spec{WritableDir: t.TempDir()}); err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	for i := range orig {
		if argv[i] != orig[i] {
			t.Fatalf("Wrap mutated input argv: %v vs %v", argv, orig)
		}
	}
}

func TestSandboxWrapRejectsBadSpec(t *testing.T) {
	sb := NewSandbox()
	if err := sb.Available(); err != nil {
		t.Skipf("no sandbox: %v", err)
	}
	if _, err := sb.Wrap(nil, Spec{WritableDir: t.TempDir()}); err == nil {
		t.Error("empty argv should error")
	}
	if _, err := sb.Wrap([]string{"echo"}, Spec{}); err == nil {
		t.Error("missing WritableDir should error — a sandbox with no confinement is not a sandbox")
	}
}

// TestUnavailableSandboxRefusesToWrap pins the fail-closed contract: the
// no-mechanism fallback must NOT return the argv unwrapped, because a caller
// that ignored Available would then run untrusted input unconfined.
func TestUnavailableSandboxRefusesToWrap(t *testing.T) {
	u := unavailableSandbox{reason: "test"}
	if err := u.Available(); !errors.Is(err, ErrSandboxUnavailable) {
		t.Errorf("Available should wrap ErrSandboxUnavailable, got %v", err)
	}
	got, err := u.Wrap([]string{"echo", "hi"}, Spec{WritableDir: "/tmp"})
	if err == nil {
		t.Fatal("Wrap must refuse when no sandbox is available")
	}
	if got != nil {
		t.Errorf("Wrap must not return an argv when unavailable, got %v", got)
	}
}

// TestRunFailsClosedWithoutSandbox is the platform-independent proof of the
// fail-closed contract — it is what a Windows/BSD host (or a Linux host without
// unprivileged user namespaces) does. Run MUST refuse rather than execute
// unconfined, and the error must name the reason so an operator can act on it.
//
// This is the behavior we cannot reach via Docker (Windows containers need a
// Windows host), so it is asserted directly by injecting the unavailable state.
func TestRunFailsClosedWithoutSandbox(t *testing.T) {
	skipOnWindows(t)
	r, err := New(5*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a host with no working sandbox (no operator opt-out).
	r.sandbox = unavailableSandbox{reason: "simulated: no mechanism on this host"}
	r.sandboxErr = r.sandbox.Available()

	_, _, runErr := r.Run(context.Background(), []string{"echo", "should-not-run"}, []byte("x"), true)
	if runErr == nil {
		t.Fatal("Run must refuse to execute when confinement is required but unavailable")
	}
	if !errors.Is(runErr, ErrSandboxUnavailable) {
		t.Errorf("error should wrap ErrSandboxUnavailable so callers can branch; got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "refusing to run") {
		t.Errorf("error should say it refused, got %q", runErr)
	}
	if !strings.Contains(r.Describe(), "unavailable") {
		t.Errorf("Describe should tell the operator commands will refuse to run, got %q", r.Describe())
	}
}

// TestRunTimeoutLeavesNoOrphans pins the process-group kill. Without it, a
// converter that spawns a helper (pandoc → a PDF engine) leaves the helper
// running after the deadline: exec.CommandContext signals only the direct child.
// Verified before the fix — a detached grandchild outlived the timeout and wrote
// its marker afterwards.
func TestRunTimeoutLeavesNoOrphans(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "orphan_marker")

	// The sandbox confines writes to its own temp dir, not ours, so this probe
	// runs unsandboxed — it is testing the process-group kill, not confinement.
	r, err := New(1*time.Second, 1<<20, WithSandboxDisabled())
	if err != nil {
		t.Fatal(err)
	}

	// Grandchild writes the marker 3s after the parent is killed at 1s.
	script := "(sleep 3; echo alive > " + marker + ") & sleep 60"
	_, _, runErr := r.Run(context.Background(), []string{"sh", "-c", script}, nil, false)
	if runErr == nil {
		t.Fatal("expected a timeout error")
	}

	// Wait past the grandchild's write time. If the group kill worked, nothing
	// is left alive to create the marker.
	time.Sleep(4 * time.Second)
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("ORPHAN SURVIVED the timeout: a descendant outlived the deadline " +
			"and kept running — the process-group kill is not working")
	}
}

// TestMaxConcurrentSerializes pins the aggregate bound: per-command limits cap
// one process, but without a concurrency bound N simultaneous requests multiply
// resource use N-fold — the cheapest way to exhaust the host. Verified by
// timing: with one slot, two 300ms commands cannot overlap.
func TestMaxConcurrentSerializes(t *testing.T) {
	skipOnWindows(t)
	r, err := New(10*time.Second, 1<<20, WithSandboxDisabled(), WithMaxConcurrent(1))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			_, _, _ = r.Run(context.Background(), []string{"sleep", "0.3"}, nil, false)
		})
	}
	wg.Wait()
	if elapsed := time.Since(start); elapsed < 550*time.Millisecond {
		t.Errorf("with 1 slot two 300ms commands should serialize (>=600ms); took %v", elapsed)
	}
}

// TestRunProceedsWhenSandboxExplicitlyDisabled pins the operator escape hatch:
// with confinement disabled, a command runs even where no mechanism exists.
func TestRunProceedsWhenSandboxExplicitlyDisabled(t *testing.T) {
	skipOnWindows(t)
	// No field poking: WithSandboxDisabled must select disabledSandbox on its own,
	// which is the behavior an operator actually gets.
	r, err := New(5*time.Second, 1<<20, WithSandboxDisabled())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.sandbox.(disabledSandbox); !ok {
		t.Fatalf("WithSandboxDisabled must select disabledSandbox, got %T", r.sandbox)
	}

	out, _, runErr := r.Run(context.Background(), []string{"echo", "ran"}, []byte("x"), true)
	if runErr != nil {
		t.Fatalf("disabled sandbox should allow the run: %v", runErr)
	}
	if !strings.Contains(string(out), "ran") {
		t.Errorf("expected command output, got %q", out)
	}
	if !strings.Contains(r.Describe(), "DISABLED") {
		t.Errorf("Describe must make an operator opt-out obvious in the log, got %q", r.Describe())
	}
}

// TestDisabledSandboxIsUnreachableWithoutOptOut is the safety property that
// justifies having two no-op-ish types instead of one: a host with NO mechanism
// must never end up with the pass-through implementation. Only an explicit
// WithSandboxDisabled may select it — otherwise "unsandboxable host" would
// silently become "runs unconfined".
func TestDisabledSandboxIsUnreachableWithoutOptOut(t *testing.T) {
	r, err := New(5*time.Second, 1<<20) // no opt-out
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.sandbox.(disabledSandbox); ok {
		t.Fatal("disabledSandbox must NEVER be selected without an explicit opt-out")
	}
}

// TestDisabledSandboxPassesArgvThrough pins the pass-through contract, and that
// it still validates the spec (a missing writable dir is a programming error
// regardless of whether confinement is on).
func TestDisabledSandboxPassesArgvThrough(t *testing.T) {
	d := disabledSandbox{}
	if err := d.Available(); err != nil {
		t.Errorf("disabled sandbox is a working configuration, got %v", err)
	}
	argv := []string{"echo", "hi"}
	got, err := d.Wrap(argv, Spec{WritableDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if len(got) != len(argv) || got[0] != "echo" || got[1] != "hi" {
		t.Errorf("argv should pass through untouched, got %v", got)
	}
	if _, err := d.Wrap(argv, Spec{}); err == nil {
		t.Error("missing WritableDir should still error")
	}
}
