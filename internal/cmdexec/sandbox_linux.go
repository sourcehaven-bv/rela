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

// readOnlyPaths is what a document converter is allowed to READ: its own
// binaries, shared libraries, and font/TeX data. Everything else — the project
// directory, /root, /home, .rela secrets, /etc/passwd — simply is not present
// inside the mount namespace.
//
// No TLS trust store: --unshare-all leaves the command with no network, so CA
// certificates would be read surface bought for nothing.
//
// Deliberately excludes /etc wholesale, keeping only the few subpaths converters
// genuinely consult.
var readOnlyPaths = []string{
	"/usr",          // binaries, libraries, fonts, TeX trees
	"/bin", "/sbin", // usr-merge symlink targets on older layouts
	"/lib", "/lib64", "/lib32",
	"/etc/fonts",        // fontconfig
	"/etc/alternatives", // Debian binary indirection
	"/var/lib/texmf",    // TeX Live generated config
	"/var/lib/fontconfig",
}

// Deliberately NOT in the list:
//
//   - /opt — unmanaged, arbitrary-vendor territory. Whatever an admin unpacked
//     there (license files, credentials, application data) would become readable
//     by a converter, for the speculative benefit of a tool that might live
//     there. An operator who really does install a converter under /opt should
//     extend this list knowingly rather than get it by default.
//   - /etc (wholesale) — passwd, shadow, and rela's own config live there.
//   - CA certificates — there is no network inside the sandbox, so a trust store
//     is read surface bought for nothing.

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
		return fmt.Errorf("%w: bwrap cannot create a user namespace on this host "+
			"(kernel.unprivileged_userns_clone=0, or Ubuntu 23.10+ "+
			"kernel.apparmor_restrict_unprivileged_userns=1): %s", ErrSandboxUnavailable, msg)
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
	// are therefore restricted to what a converter genuinely needs, so a path
	// outside the list does not merely fail permission-wise, it does not exist.
	//
	// -try variants: these paths differ across distros (no /lib64 on some, no
	// /etc/ssl in a minimal image); a missing one must not break the sandbox.
	for _, p := range readOnlyPaths {
		wrapped = append(wrapped, "--ro-bind-try", p, p)
	}
	wrapped = append(wrapped,
		"--proc", "/proc",
		"--dev", "/dev",
		// NOTE: --tmpfs /tmp must come BEFORE the writable bind below. The temp
		// dir usually lives under /tmp, and bwrap applies operations in argv
		// order — mounting the tmpfs afterwards would hide it. Order is
		// load-bearing; TestSandboxWritableDirUnderTmp pins it.
		"--tmpfs", "/tmp",
		"--bind", spec.WritableDir, spec.WritableDir,
		"--chdir", spec.WritableDir,
	)
	if spec.Network {
		// Explicit opt-in; --share-net re-joins the host network namespace.
		wrapped = append(wrapped, "--share-net")
	}
	wrapped = append(wrapped, "--")
	wrapped = append(wrapped, argv...)
	return wrapped, nil
}
