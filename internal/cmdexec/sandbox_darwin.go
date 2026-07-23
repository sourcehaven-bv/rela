package cmdexec

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// newPlatformSandbox returns the macOS backend, built on sandbox-exec (Seatbelt).
//
// sandbox-exec is formally deprecated by Apple (it says so in its own man page)
// but still ships and still works; there is no supported replacement for
// confining a headless child process — App Sandbox needs code-signed
// entitlements. Treat macOS as a best-effort/development tier: it genuinely
// blocks egress and writes (verified by TestSandboxDarwinBlocksEgress), but the
// SBPL profile language is undocumented and unversioned, so Linux is the tier to
// rely on in production.
func newPlatformSandbox() Sandbox { return darwinSandbox{} }

type darwinSandbox struct{}

func (darwinSandbox) Name() string { return "sandbox-exec" }

// Available checks that sandbox-exec exists AND actually enforces, by running a
// trivial command under a deny-network profile. Merely finding the binary is not
// enough — a profile that fails to apply looks exactly like one that works.
func (d darwinSandbox) Available() error {
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		return fmt.Errorf("%w: sandbox-exec not found on PATH: %w", ErrSandboxUnavailable, err)
	}
	// A minimal profile over /usr/bin/true: if SBPL parsing or application
	// breaks on this host, this fails and we report unavailable.
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sandbox-exec", "-p",
		"(version 1)(allow default)(deny network*)", "/usr/bin/true")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: sandbox-exec probe failed: %w: %s",
			ErrSandboxUnavailable, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Wrap prefixes argv with sandbox-exec carrying an inline SBPL profile.
//
// Profile construction has three non-obvious requirements, each learned the hard
// way; changing this needs a re-run of the sandbox tests:
//
//  1. ALLOW-DEFAULT, not deny-default. `(deny default)` SIGABRTs before main —
//     dyld cannot map the shared cache — so the profile denies specific classes.
//  2. LAST MATCH WINS. The broad `(deny file-write*)` must come BEFORE the
//     narrow `(allow file-write* (subpath …))`, or the allow is overridden and
//     the command cannot write its own output.
//  3. SYMLINKS RESOLVED. os.MkdirTemp yields /var/folders/…, which the kernel
//     evaluates as /private/var/folders/…. An unresolved path in the profile
//     silently denies every write.
func (d darwinSandbox) Wrap(argv []string, spec Spec) ([]string, error) {
	if err := validateSpec(argv, spec); err != nil {
		return nil, err
	}
	// (3) resolve symlinks — /var → /private/var.
	writable, err := filepath.EvalSymlinks(spec.WritableDir)
	if err != nil {
		return nil, fmt.Errorf("cmdexec: sandbox: resolve writable dir: %w", err)
	}

	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(allow default)\n") // (1)
	if !spec.Network {
		b.WriteString("(deny network*)\n")
	}
	b.WriteString("(deny file-write*)\n") // (2) broad deny first…
	fmt.Fprintf(&b, "(allow file-write* (subpath %s))\n", sbplString(writable))
	// Converters legitimately write to the standard sinks; denying these breaks
	// ordinary logging and stdout capture rather than adding safety.
	b.WriteString(`(allow file-write-data (literal "/dev/null") ` +
		`(literal "/dev/stdout") (literal "/dev/stderr") (literal "/dev/dtracehelper"))` + "\n")

	wrapped := make([]string, 0, len(argv)+3)
	wrapped = append(wrapped, "sandbox-exec", "-p", b.String())
	wrapped = append(wrapped, argv...)
	return wrapped, nil
}

// sbplString quotes a path as an SBPL string literal. SBPL uses TCL-ish quoting;
// backslash and double-quote are the characters that must be escaped. Paths here
// are runner-owned temp dirs (never request input), so this is defense in depth.
func sbplString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
