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
		"--ro-bind", "/", "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--bind", spec.WritableDir, spec.WritableDir,
		"--chdir", spec.WritableDir,
	}
	if spec.Network {
		// Explicit opt-in; --share-net re-joins the host network namespace.
		wrapped = append(wrapped, "--share-net")
	}
	wrapped = append(wrapped, "--")
	wrapped = append(wrapped, argv...)
	return wrapped, nil
}
