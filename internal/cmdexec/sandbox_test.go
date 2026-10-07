package cmdexec

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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

// TestSandboxBlocksReadOutsideAllowlist pins the file-disclosure control. It is
// the counterpart to the egress test: a converter can be coerced into READING a
// file and embedding it in the output — a markdown body with a raw LaTeX block
// (\input{/etc/passwd}) makes the TeX engine do exactly that, and pandoc's own
// --sandbox does not prevent it.
//
// Linux confines reads to an allowlist so the file is not even present. macOS
// does NOT (see the KNOWN LIMITATION on darwinSandbox), so this skips there
// rather than asserting a guarantee that platform does not provide.
func TestSandboxBlocksReadOutsideAllowlist(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("read confinement is a Linux-only guarantee; see darwinSandbox docs")
	}
	sb := requireWorkingSandbox(t)

	// A secret outside the writable dir and outside every allowlisted path.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	secret := filepath.Join(home, ".rela-sandbox-read-canary")
	const canary = "SANDBOX-READ-CANARY"
	if writeErr := os.WriteFile(secret, []byte(canary), 0o600); writeErr != nil {
		t.Skipf("cannot create canary: %v", writeErr)
	}
	defer os.Remove(secret)

	// Control: unsandboxed the read succeeds, so a "blocked" result is meaningful.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctrl, ctrlErr := exec.CommandContext(ctx, "cat", secret).CombinedOutput()
	if ctrlErr != nil || !strings.Contains(string(ctrl), canary) {
		t.Fatalf("control read failed; test would be vacuous: err=%v out=%q", ctrlErr, ctrl)
	}

	out, _ := runWrapped(t, sb, t.TempDir(), "cat", secret)
	if strings.Contains(out, canary) {
		t.Errorf("SANDBOX DID NOT BLOCK READ — a crafted document could exfiltrate "+
			"server-readable files into an export. output=%q", out)
	}
}

// TestSandboxExtraReadOnlyReachesBoundSocket pins the ClamAV case: a scanner
// daemon's unix socket lives outside the sandbox's default mount view, so a scan
// command cannot reach it — unless its path is bound via WithExtraReadOnly. The
// bind grants reachability WITHOUT network egress (a socket is a filesystem
// object). Verified by connecting to a listener over a bound socket path.
//
// On macOS the same holds by a different route: the connect is a network
// operation there, so the profile's network deny blocks it until the path is
// allowed for outbound unix-socket connects.
func TestSandboxExtraReadOnlyReachesBoundSocket(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("no sandbox backend on this platform")
	}
	python := pythonPath(t)

	// A unix socket in a dir that is NOT in the default allowlist. Under /tmp
	// rather than t.TempDir: macOS caps a socket path at 104 bytes, which the
	// per-test temp dir under /var/folders exceeds.
	sockDir, err := os.MkdirTemp("/tmp", "sock") //nolint:usetesting // socket path length, see above
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	sockPath := filepath.Join(sockDir, "clamd.ctl")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("cannot listen on unix socket: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			_, _ = c.Write([]byte("PONG"))
			c.Close()
		}
	}()

	connect := "import socket\n" +
		"s=socket.socket(socket.AF_UNIX)\n" +
		"try:\n" +
		"    s.connect(" + pyStr(sockPath) + ")\n" +
		"    print('CONNECTED')\n" +
		"except Exception as e:\n" +
		"    print('UNREACHABLE', type(e).__name__)\n"

	// Without the bind: unreachable (the socket path is not in the mount view).
	r, err := New(20*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.sandbox.Available(); err != nil {
		t.Skipf("no working sandbox: %v", err)
	}
	out, _, _ := r.Run(context.Background(), []string{python, "-c", connect}, nil, true)
	if strings.Contains(string(out), "CONNECTED") {
		t.Fatalf("socket should be UNREACHABLE without a bind (test is vacuous otherwise): %q", out)
	}

	// With the bind: reachable, and still no network (unix socket, not egress).
	rb, berr := New(20*time.Second, 1<<20, WithExtraReadOnly(sockDir))
	if berr != nil {
		t.Fatal(berr)
	}
	outb, _, _ := rb.Run(context.Background(), []string{python, "-c", connect}, nil, true)
	if !strings.Contains(string(outb), "CONNECTED") {
		t.Errorf("bound socket should be reachable (this is how clamd works under "+
			"the sandbox): %q", outb)
	}
}

// TestWithExtraReadOnlySkipsNonAbsolute pins that a typo'd (empty or relative)
// bind path is dropped rather than silently becoming a useless -try no-op, so a
// misconfigured read path is visible in the log, not invisible.
func TestWithExtraReadOnlySkipsNonAbsolute(t *testing.T) {
	r, err := New(time.Second, 1<<20,
		WithSandboxDisabled(), // no platform sandbox needed to inspect the option effect
		WithExtraReadOnly("/abs/ok.sock", "", "relative/path.sock", "/another/ok"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/abs/ok.sock", "/another/ok"}
	if len(r.extraReadOnly) != len(want) {
		t.Fatalf("got %v, want only absolute paths %v", r.extraReadOnly, want)
	}
	for i, p := range want {
		if r.extraReadOnly[i] != p {
			t.Errorf("index %d: got %q, want %q", i, r.extraReadOnly[i], p)
		}
	}
}

// TestSandboxErrNilOnOperatorOptOut pins that an explicit opt-out is not
// reported as a sandbox failure. The composition root warns "a scan is
// configured but cannot run" on a non-nil SandboxErr; an operator who ran with
// RELA_UNCONFINED_COMMANDS=1 has accepted unconfined execution, and their
// commands DO run, so surfacing that as a failure would be a false alarm. This
// holds by construction — New takes the opt-out branch before consulting the
// platform probe — and this test keeps it that way.
func TestSandboxErrNilOnOperatorOptOut(t *testing.T) {
	r, err := New(time.Second, 1<<20, WithSandboxDisabled())
	if err != nil {
		t.Fatal(err)
	}
	if got := r.SandboxErr(); got != nil {
		t.Errorf("SandboxErr = %v, want nil (opt-out is an accepted risk, not a failure)", got)
	}
}

// TestParseReadPaths pins the RELA_SANDBOX_*_READ_PATHS format: PATH-style
// separators, with empty entries (a doubled or trailing separator) dropped.
func TestParseReadPaths(t *testing.T) {
	sep := string(filepath.ListSeparator)
	got := ParseReadPaths("/etc/fonts" + sep + sep + " /etc/paperspecs " + sep)
	want := []string{"/etc/fonts", "/etc/paperspecs"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := ParseReadPaths(""); len(got) != 0 {
		t.Errorf("empty input: got %v, want none", got)
	}
}

// TestHostReadOnlyReachesEveryRunner pins that the operator's list is bound in
// every runner New builds, ahead of a runner's own extra paths, and that
// non-absolute entries are dropped. Every runner matters: export, document
// commands and attachment transforms each build their own, and a converter that
// cannot read fontconfig or libpaper's paper sizes fails the whole export.
func TestHostReadOnlyReachesEveryRunner(t *testing.T) {
	t.Cleanup(func() { SetHostReadOnly(PurposeTransform, nil) })

	accepted := SetHostReadOnly(PurposeTransform, []string{"/etc/fonts", "relative/x", "/etc/paperspecs/"})
	if want := []string{"/etc/fonts", "/etc/paperspecs"}; !slices.Equal(accepted, want) {
		t.Fatalf("accepted %v, want %v", accepted, want)
	}

	r, err := New(time.Second, 1<<20, WithSandboxDisabled(), WithExtraReadOnly("/run/x.sock"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/etc/fonts", "/etc/paperspecs", "/run/x.sock"}
	if got := r.ExtraReadOnly(); !slices.Equal(got, want) {
		t.Errorf("runner binds %v, want %v", got, want)
	}

	SetHostReadOnly(PurposeTransform, nil)
	r, err = New(time.Second, 1<<20, WithSandboxDisabled())
	if err != nil {
		t.Fatal(err)
	}
	if got := r.ExtraReadOnly(); len(got) != 0 {
		t.Errorf("after reset, runner binds %v; want none", got)
	}
}

// TestHostReadOnlyIsPerPurpose pins the isolation between purposes: a scan
// runner binds only the scan list and a transform runner only the transform
// list. A converter fed untrusted content must not reach the clamd socket, and
// a runner that names no purpose is a transform runner.
func TestHostReadOnlyIsPerPurpose(t *testing.T) {
	t.Cleanup(func() {
		SetHostReadOnly(PurposeScan, nil)
		SetHostReadOnly(PurposeTransform, nil)
	})
	scan := []string{"/run/clamav/clamd.ctl", "/etc/clamav/clamd.conf"}
	transform := []string{"/etc/paperspecs", "/var/lib/texmf"}
	SetHostReadOnly(PurposeScan, scan)
	SetHostReadOnly(PurposeTransform, transform)

	cases := []struct {
		name string
		opts []Option
		want []string
	}{
		{"scan", []Option{WithPurpose(PurposeScan)}, scan},
		{"transform", []Option{WithPurpose(PurposeTransform)}, transform},
		{"no purpose", nil, transform},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := New(time.Second, 1<<20, append(tc.opts, WithSandboxDisabled())...)
			if err != nil {
				t.Fatal(err)
			}
			if got := r.ExtraReadOnly(); !slices.Equal(got, tc.want) {
				t.Errorf("runner binds %v, want %v", got, tc.want)
			}
		})
	}
}

// TestApplyHostEnv pins the wiring the CLI and rela-desktop share: every host
// setting comes from the environment in one call, so a composition root cannot
// apply one and miss another.
func TestApplyHostEnv(t *testing.T) {
	t.Cleanup(func() {
		SetHostReadOnly(PurposeScan, nil)
		SetHostReadOnly(PurposeTransform, nil)
		SetUnconfinedByDefault(false)
	})
	t.Setenv(EnvSandboxTransformReadPaths, "/etc/paperspecs::/var/lib/texmf/")
	t.Setenv(EnvSandboxScanReadPaths, "/run/clamav/clamd.ctl")
	t.Setenv(EnvUnconfinedCommands, "1")

	ApplyHostEnv()
	if got, want := HostReadOnly(PurposeTransform), []string{"/etc/paperspecs", "/var/lib/texmf"}; !slices.Equal(got, want) {
		t.Errorf("HostReadOnly(PurposeTransform) = %v, want %v", got, want)
	}
	if got, want := HostReadOnly(PurposeScan), []string{"/run/clamav/clamd.ctl"}; !slices.Equal(got, want) {
		t.Errorf("HostReadOnly(PurposeScan) = %v, want %v", got, want)
	}
	if !unconfinedDefault() {
		t.Error("RELA_UNCONFINED_COMMANDS=1 not applied")
	}
}

// TestSetHostReadOnlyRejectsPathsThatUndoTheSandbox pins the paths an operator
// cannot list. Operator binds are applied after the sandbox's own /proc, /dev,
// /tmp and writable-dir mounts, so each of these would replace one of them,
// expose the whole host, or hand over every daemon socket or data directory.
func TestSetHostReadOnlyRejectsPathsThatUndoTheSandbox(t *testing.T) {
	t.Cleanup(func() { SetHostReadOnly(PurposeTransform, nil) })
	// A nested TMPDIR, so "a parent of the temp dir" is a real case on Linux
	// CI too, where the default /tmp's parent is already refused as "/".
	tmp := filepath.Join(t.TempDir(), "nested", "tmp")
	t.Setenv("TMPDIR", tmp)

	rejected := []string{
		"/", "/etc", "/etc/",
		"/proc", "/proc/self", "/dev", "/dev/shm",
		"/tmp", tmp, filepath.Dir(tmp),
		"/run", "/var/run", "/var", "/var/lib", "/home", "/root",
	}
	for _, p := range rejected {
		if got := SetHostReadOnly(PurposeTransform, []string{p}); len(got) != 0 {
			t.Errorf("SetHostReadOnly(%q) accepted %v; want it rejected", p, got)
		}
		r, err := New(time.Second, 1<<20, WithSandboxDisabled(), WithExtraReadOnly(p))
		if err != nil {
			t.Fatal(err)
		}
		if got := r.ExtraReadOnly(); len(got) != 0 {
			t.Errorf("WithExtraReadOnly(%q) bound %v; want it rejected", p, got)
		}
	}

	// Single files and sockets inside protected directories stay allowed, as
	// do names that merely share a prefix with one.
	allowed := []string{
		"/etc/paperspecs", "/var/lib/texmf", "/var/run/clamav/clamd.ctl",
		"/tmp/clamd.sock", "/devices", "/procfs", "/etcetera",
	}
	if got := SetHostReadOnly(PurposeTransform, allowed); !slices.Equal(got, allowed) {
		t.Errorf("SetHostReadOnly(%v) = %v; want all accepted", allowed, got)
	}
}

// TestSetHostReadOnlyChecksSymlinkTargets pins that the check follows
// symlinks: bwrap binds a symlink's target, and the macOS backend resolves
// paths before writing its profile, so a link to "/" or to /dev must be
// refused like the target itself. A path that does not exist yet is resolved
// through its existing parent.
func TestSetHostReadOnlyChecksSymlinkTargets(t *testing.T) {
	t.Cleanup(func() { SetHostReadOnly(PurposeTransform, nil) })
	dir := t.TempDir()
	for name, target := range map[string]string{"root": "/", "dev": "/dev", "etc": "/etc"} {
		link := filepath.Join(dir, name)
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if got := SetHostReadOnly(PurposeTransform, []string{link}); len(got) != 0 {
			t.Errorf("symlink %s -> %s accepted", link, target)
		}
		if got := SetHostReadOnly(PurposeTransform, []string{filepath.Join(link, "not-yet")}); target == "/dev" && len(got) != 0 {
			t.Errorf("future path under symlink %s -> /dev accepted", link)
		}
	}
}

// TestSetHostReadOnlyDropsDuplicates keeps bwrap from binding a path twice.
func TestSetHostReadOnlyDropsDuplicates(t *testing.T) {
	t.Cleanup(func() { SetHostReadOnly(PurposeTransform, nil) })
	got := SetHostReadOnly(PurposeTransform, []string{"/etc/paperspecs", "/etc/paperspecs/", "/var/lib/texmf"})
	if want := []string{"/etc/paperspecs", "/var/lib/texmf"}; !slices.Equal(got, want) {
		t.Errorf("SetHostReadOnly = %v, want %v", got, want)
	}
}

// TestCheckProjectNotExposed pins that no operator path may be, contain, or lie
// inside the project directory, including through a symlink.
func TestCheckProjectNotExposed(t *testing.T) {
	t.Cleanup(func() {
		SetHostReadOnly(PurposeScan, nil)
		SetHostReadOnly(PurposeTransform, nil)
	})
	base := t.TempDir()
	project := filepath.Join(base, "srv", "project")
	if err := os.MkdirAll(filepath.Join(project, ".rela"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(project, link); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		paths   []string
		wantErr bool
	}{
		{"unrelated paths", []string{"/etc/paperspecs", "/var/lib/texmf"}, false},
		{"the project itself", []string{project}, true},
		{"a parent of the project", []string{filepath.Join(base, "srv")}, true},
		{"inside the project", []string{filepath.Join(project, ".rela")}, true},
		{"a symlink to the project", []string{link}, true},
		{"a sibling", []string{filepath.Join(base, "srv", "other")}, false},
	}
	// Every purpose's list is checked: a scan path inside the project would
	// expose it as surely as a transform path.
	for _, purpose := range []Purpose{PurposeTransform, PurposeScan} {
		for _, tc := range cases {
			t.Run(purpose.String()+"/"+tc.name, func(t *testing.T) {
				// Bypass SetHostReadOnly: a TempDir path may itself be refused as
				// lying under the temp dir's parent on some hosts, and this test is
				// about the project check alone.
				paths := slices.Clone(tc.paths)
				hostReadOnly[purpose].Store(&paths)
				t.Cleanup(func() { hostReadOnly[purpose].Store(nil) })
				err := CheckProjectNotExposed(project)
				if (err != nil) != tc.wantErr {
					t.Errorf("CheckProjectNotExposed = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	}
	if err := CheckProjectNotExposed(""); err != nil {
		t.Errorf("empty project root: %v", err)
	}
}

// TestWrapRefusesBindOverWritableDir pins that a read-only bind containing the
// command's writable dir is refused rather than silently making it read-only.
// unsafeReadPath only knows the OS temp dir; WithTempDir can point elsewhere.
func TestWrapRefusesBindOverWritableDir(t *testing.T) {
	spec := Spec{WritableDir: "/srv/scratch/run1", ExtraReadOnly: []string{"/srv/scratch"}}
	if err := validateSpec([]string{"true"}, spec); err == nil {
		t.Error("bind over the writable dir accepted")
	}
	spec.ExtraReadOnly = []string{"/srv/scratch/run1/sock", "/srv/other"}
	if err := validateSpec([]string{"true"}, spec); err != nil {
		t.Errorf("unrelated binds refused: %v", err)
	}
}

// TestSandboxWritableDirUnderTmp pins an argv-ordering dependency in the Linux
// backend: --tmpfs /tmp must precede the --bind of the writable dir. The run's
// temp dir usually lives under /tmp, and bwrap applies operations in order — a
// tmpfs mounted afterwards would hide it and every {out}-using transform would
// silently produce nothing. t.TempDir() does not necessarily sit under /tmp, so
// this exercises the case explicitly.
func TestSandboxWritableDirUnderTmp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("bwrap-specific ordering")
	}
	sb := requireWorkingSandbox(t)

	// t.TempDir() is NOT necessarily under /tmp, which is the whole point here.
	dir, err := os.MkdirTemp("/tmp", "rela-sandbox-order-") //nolint:usetesting // must be under /tmp
	if err != nil {
		t.Skipf("cannot create a dir under /tmp: %v", err)
	}
	defer os.RemoveAll(dir)

	target := filepath.Join(dir, "out.txt")
	out, runErr := runWrapped(t, sb, dir, "sh", "-c", "echo written > "+target+" && cat "+target)
	if runErr != nil {
		t.Fatalf("writing inside a /tmp-based writable dir failed (--tmpfs likely "+
			"shadows the bind — check argv order): err=%v out=%q", runErr, out)
	}
	if !strings.Contains(out, "written") {
		t.Errorf("expected the write to land and read back; got %q", out)
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

// TestWaitDelayLingeringChildIsSuccessOnOutFile pins that a command which
// completes and writes {out}, but leaves a detached child holding an output pipe
// open past waitDelay, is treated as SUCCESS rather than a spurious ErrWaitDelay
// failure. This is the canonical pandoc→PDF case: the engine grandchild lingers,
// yet the PDF is already on disk. Verified to fail (returns an error) before the
// ErrWaitDelay carve-out in execute().
func TestWaitDelayLingeringChildIsSuccessOnOutFile(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	// Timeout well above waitDelay (2s) so the DeadlineExceeded path does NOT fire
	// — we specifically want the WaitDelay path with ctx.Err()==nil.
	r, err := New(30*time.Second, 1<<20, WithSandboxDisabled())
	if err != nil {
		t.Fatal(err)
	}
	// Write {out}, then background a child that inherits stdout and sleeps past
	// waitDelay before the parent exits 0. The child keeps the stdout pipe open,
	// forcing Wait to return ErrWaitDelay ~2s after the parent reaps.
	// {out} must be its OWN argv element to be substituted (it maps to $1 here),
	// matching the transform contract — it is not textually expanded inside a word.
	script := `echo done > "$1"; sleep 5 & exit 0`
	out, usedOut, runErr := r.Run(context.Background(),
		[]string{"sh", "-c", script, "sh", "{out}"}, nil, true)
	if runErr != nil {
		t.Fatalf("lingering child on the {out} path must be success, got: %v", runErr)
	}
	if !usedOut {
		t.Fatal("expected the {out} temp-file path")
	}
	if strings.TrimSpace(string(out)) != "done" {
		t.Errorf("out-file content = %q, want \"done\"", out)
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

// TestUnconfinedByDefault pins the host-level knob the composition roots set:
// once enabled, every new runner opts out of confinement without each call site
// passing WithSandboxDisabled — so the CLI and server agree from one source.
func TestUnconfinedByDefault(t *testing.T) {
	skipOnWindows(t)
	t.Cleanup(func() { SetUnconfinedByDefault(false) })

	// Default: a runner built with no options is confined (disabledSandbox only
	// when the host can't sandbox; here we assert it is NOT the disabled one when
	// the mechanism works).
	SetUnconfinedByDefault(false)
	r, err := New(5*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.sandbox.(disabledSandbox); ok {
		t.Error("without the host knob, a runner must not be unconfined by default")
	}

	// With the knob set, a runner built with no options is unconfined.
	SetUnconfinedByDefault(true)
	r2, err := New(5*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r2.sandbox.(disabledSandbox); !ok {
		t.Errorf("host knob set → runner should be unconfined, got %T", r2.sandbox)
	}
	// And a command actually runs (would refuse on an unsandboxable host otherwise).
	if _, _, runErr := r2.Run(context.Background(), []string{"echo", "ok"}, nil, false); runErr != nil {
		t.Errorf("unconfined runner should execute: %v", runErr)
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
