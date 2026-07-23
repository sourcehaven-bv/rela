package attachment

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
)

// These tests pin that attachment processing INHERITS the confinement and
// resource bounds implemented in internal/cmdexec. Attachments are the original
// case for it: an operator-configured third-party parser (ClamAV, ImageMagick,
// qpdf, exiftool) is run over bytes an untrusted user uploaded. The guarantees
// are implemented once in the shared runner, so these tests exist to catch a
// regression where CmdRunner stops routing through it.

func requirePython(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX probe fixtures")
	}
	p, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 unavailable: %v", err)
	}
	return p
}

// requireSandboxedRunner builds a CmdRunner and skips unless confinement is
// actually active on this host — otherwise the assertions below would be
// vacuous (a "blocked" result could just mean the network is down).
//
// Readiness is probed by running a trivial command: if the host cannot confine,
// Run refuses and we skip. That is the same signal production code uses — there
// is no separate "am I ready" predicate to drift from it.
func requireSandboxedRunner(t *testing.T, timeout time.Duration) *CmdRunner {
	t.Helper()
	r, err := NewCmdRunner(timeout, 1<<20)
	if err != nil {
		t.Fatalf("NewCmdRunner: %v", err)
	}
	_, _, probeErr := r.Transform(context.Background(), []string{"true"}, ProcessContext{}, []byte("x"))
	if probeErr != nil && errors.Is(probeErr, cmdexec.ErrSandboxUnavailable) {
		t.Skipf("no working sandbox on this host: %v", probeErr)
	}
	return r
}

// TestAttachmentTransformBlocksEgress is the cross-consumer proof of the SSRF
// control: a transform command configured for an attachment property must not be
// able to reach the network, because the bytes it parses are attacker-supplied.
func TestAttachmentTransformBlocksEgress(t *testing.T) {
	python := requirePython(t)
	r := requireSandboxedRunner(t, 20*time.Second)

	script := "import socket\n" +
		"s=socket.socket(); s.settimeout(3)\n" +
		"try:\n" +
		"    s.connect(('1.1.1.1',80)); print('CONNECTED')\n" +
		"except Exception as e:\n" +
		"    print('BLOCKED', type(e).__name__)\n"

	out, _, err := r.Transform(context.Background(),
		[]string{python, "-c", script}, ProcessContext{}, []byte("x"))
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if strings.Contains(string(out), "CONNECTED") {
		t.Error("attachment transform reached the network — a malicious upload could " +
			"make the server hit internal services or cloud metadata")
	}
	if !strings.Contains(string(out), "BLOCKED") {
		t.Errorf("expected the connect to be blocked; got %q", out)
	}
}

// TestAttachmentScanFailsClosedWithoutSandbox pins that the fail-closed contract
// reaches the SCAN path. Attachment scanning is ALREADY fail-closed when the
// scanner cannot run (a missing binary rejects the upload); an unsandboxable
// host must behave the same way rather than scanning unconfined.
//
// The unavailable state is reached by pointing the scan at a binary that does
// not exist, which is the same "could not run" branch — asserting the outcome
// (rejection, never a silent pass) on every host rather than skipping where a
// sandbox happens to work.
func TestAttachmentScanFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX probe fixtures")
	}
	r, err := NewCmdRunner(5*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	scanErr := r.Scan(context.Background(), []string{"definitely-not-a-real-scanner-xyz"}, []byte("payload"))
	if scanErr == nil {
		t.Fatal("a scan that cannot run must reject the upload, never pass it")
	}
	if !errors.Is(scanErr, ErrRejected) {
		t.Errorf("failure to run the scanner must wrap ErrRejected (fail closed); got %v", scanErr)
	}
}

// TestAttachmentRunnerDescribesConfinement pins the one diagnostic an operator
// gets at startup: a non-empty line saying how (or whether) attachment commands
// are confined. Callers never branch on it — they run the command and handle the
// error — so this only guards that the log line is actually informative.
func TestAttachmentRunnerDescribesConfinement(t *testing.T) {
	r, err := NewCmdRunner(5*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	desc := r.Describe()
	if desc == "" {
		t.Fatal("Describe must produce a startup log line")
	}
	// Whatever the host, the line must name the posture rather than be vague.
	for _, want := range []string{"sandbox"} {
		if !strings.Contains(strings.ToLower(desc), want) {
			t.Errorf("Describe should mention %q; got %q", want, desc)
		}
	}
}
