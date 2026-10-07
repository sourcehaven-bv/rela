package cmdexec

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// newPlatformSandbox returns the Linux backend, built on bubblewrap (bwrap).
//
// bwrap is the mechanism Flatpak uses: unprivileged (no setuid since bubblewrap
// 0.9 — it relies purely on unprivileged user namespaces), widely packaged, and
// designed to be driven from scripts. Overhead is ~5-20ms per invocation, an
// order of magnitude cheaper than a container per export.
func newPlatformSandbox() Sandbox { return linuxSandbox{} }

type linuxSandbox struct{}

func (linuxSandbox) Name() string { return "bubblewrap" }

// systemReadOnlyPaths is the only read access rela grants on its own: the
// directories that hold binaries and shared libraries, without which no command
// can start. Everything a particular converter or scanner needs beyond that —
// fontconfig, TeX Live config, libpaper's paper sizes, a clamd socket — is
// host-specific and comes from the operator ([SetHostReadOnly],
// RELA_SANDBOX_READ_PATHS). Everything else — the project directory, /root,
// /home, .rela secrets, /etc/passwd — simply is not present inside the mount
// namespace, as long as the operator's list does not cover it. The list is
// checked for that: [unsafeReadPath] refuses /, /etc, /home, /root, /var/lib
// and similar, and [CheckProjectNotExposed] refuses the project directory.
//
// Do not grow this list. A path a converter needs on one distro is added by the
// operator of that host, not compiled into every rela.
var systemReadOnlyPaths = []string{
	"/usr",          // binaries, libraries, fonts, TeX trees
	"/bin", "/sbin", // usr-merge symlink targets on older layouts
	"/lib", "/lib64", "/lib32",
}

// usernsFailure matches the stderr signatures of a host where bwrap exists but
// unprivileged user namespaces are unavailable — the common cases being
// kernel.unprivileged_userns_clone=0 and the Ubuntu 23.10+ AppArmor restriction
// (kernel.apparmor_restrict_unprivileged_userns=1), which surfaces as an
// RTM_NEWADDR/loopback or namespace-creation permission error.
var usernsFailure = regexp.MustCompile(`(?i)namespace|RTM_NEWADDR|loopback|permission denied|operation not permitted`)

// Available runs a real bwrap invocation rather than merely locating the binary.
// This distinction is load-bearing: bwrap exits with the CHILD's status, so a
// setup failure is indistinguishable from child success by exit code alone —
// checking only exec.LookPath would happily report a sandbox that cannot
// actually confine anything.
func (l linuxSandbox) Available() error {
	if _, err := exec.LookPath("bwrap"); err != nil {
		return fmt.Errorf("%w: bwrap (bubblewrap) not found on PATH: %w", ErrSandboxUnavailable, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bwrap", "--unshare-all", "--ro-bind", "/", "/", "/bin/true")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if usernsFailure.MatchString(msg) {
		return fmt.Errorf("%w: bwrap cannot create a user namespace on this host. "+
			"Under systemd this is usually the UNIT, not the kernel — check "+
			"RestrictNamespaces=, SystemCallFilter= and RestrictAddressFamilies= "+
			"against the unit in docs/attachment-security.md (Running under "+
			"systemd); otherwise kernel.unprivileged_userns_clone=0, or Ubuntu "+
			"23.10+ kernel.apparmor_restrict_unprivileged_userns=1: %s",
			ErrSandboxUnavailable, msg)
	}
	return fmt.Errorf("%w: bwrap probe failed: %w: %s", ErrSandboxUnavailable, err, msg)
}

// Wrap prefixes argv with a bwrap invocation confining the command to a
// read-only view of the system plus one writable directory, with no network.
//
// --unshare-all already implies --unshare-net (it expands to
// --unshare-user-try --unshare-ipc --unshare-pid --unshare-net --unshare-uts
// --unshare-cgroup-try), so egress is denied by NOT passing --share-net.
// --proc and --dev are required: most real converters break without them.
func (l linuxSandbox) Wrap(argv []string, spec Spec) ([]string, error) {
	if err := validateSpec(argv, spec); err != nil {
		return nil, err
	}

	wrapped := []string{
		"bwrap",
		"--unshare-all",     // includes --unshare-net → no egress
		"--die-with-parent", // composes with the caller's context timeout
		"--new-session",     // no controlling tty → blocks TIOCSTI injection
	}
	// READ allowlist. Binding "/" read-only would still expose every readable
	// file on the host, and a converter can be made to read one: a markdown body
	// carrying a raw LaTeX block (\input{/etc/passwd}) makes the TeX engine
	// embed that file's contents INTO the exported document — verified. Reads
	// are therefore restricted to the system paths plus the operator's list
	// (spec.ExtraReadOnly, below), so a path outside them does not merely fail
	// permission-wise, it does not exist.
	//
	// -try variants: these paths differ across distros (no /lib64 on some); a
	// missing one must not break the sandbox.
	for _, p := range systemReadOnlyPaths {
		wrapped = append(wrapped, "--ro-bind-try", p, p)
	}
	wrapped = append(wrapped,
		"--proc", "/proc",
		"--dev", "/dev",
		// --tmpfs /tmp must come BEFORE the binds below. The temp dir (and a
		// caller's extra binds) may live under /tmp, and bwrap applies operations
		// in argv order — mounting the tmpfs afterwards would hide them. Order is
		// load-bearing; TestSandboxWritableDirUnderTmp + the socket-bind test pin it.
		"--tmpfs", "/tmp",
		"--bind", spec.WritableDir, spec.WritableDir,
		"--chdir", spec.WritableDir,
	)
	// Operator-configured binds (fontconfig, a clamd socket, ...), AFTER --tmpfs
	// so a path under /tmp is not shadowed. Read-only, -try so a missing path is skipped. A
	// unix socket bound here is reachable without any network access — the network
	// namespace stays isolated.
	for _, p := range spec.ExtraReadOnly {
		wrapped = append(wrapped, "--ro-bind-try", p, p)
	}
	if spec.Network {
		// Explicit opt-in; --share-net re-joins the host network namespace.
		wrapped = append(wrapped, "--share-net")
	}
	wrapped = append(wrapped, "--")
	wrapped = append(wrapped, argv...)
	return wrapped, nil
}
