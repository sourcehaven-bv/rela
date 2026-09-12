package attachment

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// eicarSignature is the standard AV test string, assembled at runtime from two
// halves so this source file does not itself trip a scanner watching the repo.
var eicarSignature = `X5O!P%@AP[4\PZX54(P^)7CC)7}$` +
	`EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`

// requireClamd gates the suite on RELA_TEST_CLAMD, mirroring how the postgres
// suites gate on RELA_TEST_DATABASE_URL — run it with `just test-clamav`.
//
// Opt-in rather than auto-detected: silently skipping when clamd happens to be
// absent is what let the documented recipe stay broken, so a developer who asks
// for this suite gets a failure, not a skip.
func requireClamd(t *testing.T) {
	t.Helper()
	if os.Getenv("RELA_TEST_CLAMD") == "" {
		t.Skip("RELA_TEST_CLAMD not set; skipping real-clamd integration (see `just test-clamav`)")
	}
	// macOS cannot run this suite, and it is a platform limit rather than a
	// missing bind: sandbox-exec's `(deny network*)` refuses AF_UNIX connect()
	// as well as real egress, so a scan command can never reach clamd under
	// confinement. Verified directly —
	//
	//	sandbox-exec -p '(version 1)(allow default)(deny network*)' \
	//	  clamdscan --no-summary --stream f
	//	→ Could not connect to clamd on LocalSocket ...: Operation not permitted
	//
	// while the same command outside the sandbox succeeds. Linux/bwrap does not
	// have this problem: a socket is a filesystem object there, so binding it
	// grants reachability with the network namespace still isolated — which is
	// what the guide documents and what CI exercises.
	//
	// Skipping rather than failing: attachment scanning is a server-side
	// deployment concern (the docs call macOS a development tier), so this is a
	// real constraint of the platform, not of the change under test.
	if runtime.GOOS == "darwin" {
		t.Skip("macOS sandbox-exec denies AF_UNIX connect() under (deny network*), " +
			"so a confined scan command cannot reach clamd; run `just test-clamav-vm` " +
			"or use the Linux CI job")
	}
}

// documentedScanCmd is the recipe published in the attachment-security guide.
// The test asserts against the DOCUMENTED command rather than a convenient one:
// the bug this suite exists for was that the published recipe did not work, so a
// test using different arguments would have passed throughout.
var documentedScanCmd = []string{"clamdscan", "--no-summary", "--stream", "{in}"}

// clamdRunner builds the runner under test, honoring RELA_TEST_CLAMD_BINDS for
// hosts whose clamd lives outside the built-in defaults.
//
// The defaults cover the Linux distro layouts (which is what CI and the docs
// target). A Homebrew macOS install puts both the socket and clamd.conf under
// /opt/homebrew, so a developer running this locally passes those paths the same
// way an operator would use `attachments.scan_sockets:` — which means the local
// run also exercises that escape hatch rather than bypassing the sandbox.
func clamdRunner(t *testing.T) *CmdRunner {
	t.Helper()
	var opts []CmdRunnerOption
	if binds := os.Getenv("RELA_TEST_CLAMD_BINDS"); binds != "" {
		opts = append(opts, WithScannerSockets(strings.Split(binds, ",")...))
	}
	r, err := NewCmdRunner(30*time.Second, 1<<20, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestClamdScan_DocumentedRecipe runs the guide's scan command against a real
// clamd, through the real sandbox, via the same CmdRunner the upload handler
// uses.
//
// This is the gap that let TKT-ZP1EE3 ship: every other scan test substitutes a
// stub `sh -c` command, which exercises rela's plumbing but cannot observe that
// clamdscan needs its config file bound, that --fdpass cannot cross the sandbox
// namespace boundary, or that a 0700 temp dir defeats path-based scanning.
func TestClamdScan_DocumentedRecipe(t *testing.T) {
	requireClamd(t)
	skipOnWindows(t)

	r := clamdRunner(t)
	if err := r.Probe(documentedScanCmd); err != nil {
		t.Fatalf("clamdscan not runnable: %v", err)
	}
	if err := r.SandboxErr(); err != nil {
		t.Fatalf("no working sandbox, so this would not test confinement: %v", err)
	}

	for _, tc := range []struct {
		name       string
		data       []byte
		wantReject bool
	}{
		{"clean file is accepted", []byte("a perfectly ordinary file\n"), false},
		{"eicar signature is rejected", []byte(eicarSignature), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			err := r.Scan(ctx, documentedScanCmd, tc.data)
			switch {
			case tc.wantReject && err == nil:
				t.Error("infected content was accepted")
			case tc.wantReject && !errors.Is(err, ErrRejected):
				t.Errorf("got %v, want an ErrRejected-wrapping error", err)
			case !tc.wantReject && err != nil:
				// The failure mode this suite was written for: a clean file
				// rejected because the scanner could not run at all.
				t.Errorf("clean content rejected — the scanner likely could not "+
					"run (config unreadable inside the sandbox?): %v", err)
			}
		})
	}
}

// TestClamdScan_FdpassStillFails pins the reason the guide recommends --stream.
//
// If a future bubblewrap or ClamAV makes --fdpass work, this test fails — which
// is the point: the documentation explains at length why --fdpass cannot be
// used, and that explanation should be revisited rather than silently left
// wrong. A failure here is a docs task, not a defect.
func TestClamdScan_FdpassStillFails(t *testing.T) {
	requireClamd(t)
	skipOnWindows(t)

	r := clamdRunner(t)
	if err := r.SandboxErr(); err != nil {
		t.Skipf("no sandbox; --fdpass only fails BECAUSE of the namespace boundary: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fdpass := []string{"clamdscan", "--no-summary", "--fdpass", "{in}"}
	if err := r.Scan(ctx, fdpass, []byte("a perfectly ordinary file\n")); err == nil {
		t.Error("--fdpass now scans a clean file successfully under the sandbox. " +
			"The guide's \"Use --stream, not --fdpass\" section is out of date; " +
			"re-verify and update it rather than deleting this test.")
	}
}
