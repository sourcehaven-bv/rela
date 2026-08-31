---
id: TKT-ZP1EE3
type: ticket
title: 'ClamAV attachment scanning is unusable out of the box: --fdpass, missing clamd.conf bind, and hardened systemd units all break it'
kind: docs
priority: high
effort: m
status: done
---

## Description

The documented ClamAV recipe in
`docs-project/entities/guides/GUIDE-attachment-security.md` does not work on a
stock Debian install. Verified empirically in a Debian 12.15 (bookworm, arm64)
VM with `clamav-daemon` + `bubblewrap`, driving rela's real
`attachment.NewCmdRunner` path and then real HTTP uploads against `rela-server`.

Scanning is fail-closed, so this is **not** a security hole — but **every upload
is rejected** and the operator is given a misleading diagnosis. Three
independent defects, each sufficient on its own to break scanning.

### 1. `--fdpass` cannot work under bubblewrap (docs recommend it as the default)

The guide's recipe is `[clamdscan, --no-summary, --fdpass, "{in}"]`. Under
bwrap, clamd receives the fd via SCM_RIGHTS, `fstat`s it, and refuses:

```
/tmp/rela-cmdexec-XXX/in: Not a regular file ERROR   exit=2
```

Structural, not a missing bind. The guide additionally frames `--stream` as the
*remote-host* option and warns it "needs network egress" — false for a
`LocalSocket`: verified working with only `lo` inside the sandbox.

### 2. `/etc` is excluded from `readOnlyPaths`, so `clamdscan` cannot read its own config

`internal/cmdexec/sandbox_linux.go:31` excludes `/etc` wholesale (correct —
passwd/ shadow live there). But `clamdscan` parses `/etc/clamav/clamd.conf` at
startup to *find* `LocalSocket`, before connecting. Binding the socket is
necessary but not sufficient:

```
ERROR: Can't parse clamd configuration file /etc/clamav/clamd.conf   exit=2
```

The guide claims a stock install "works with no extra configuration". It does
not.

Path-based scanning is also dead for a third reason: rela's temp dir is `0700`
(`os.MkdirTemp`, `cmdexec.go:198`) and clamd runs as user `clamav`, so it cannot
open the file (`File path check failure: Permission denied`).

### 3. Hardened systemd units silently disable the sandbox

A normal `systemd-analyze`-pleasing unit starts fine, serves HTTP, and logs at
**INFO**:

```
external command confinement: sandbox unavailable (commands will refuse to run):
bwrap cannot create a user namespace on this host
(kernel.unprivileged_userns_clone=0, or Ubuntu 23.10+ ...)
```

The diagnosis is **wrong**: `kernel.unprivileged_userns_clone = 1` and bwrap
works for the same user outside systemd. Bisected via `systemd-run`:

| Directive | Result | Fix |
| --- | --- | --- |
| `RestrictNamespaces=yes` | exit=1 | `RestrictNamespaces=user mnt pid net ipc uts cgroup` (allowlist; `cgroup` required) |
| `SystemCallFilter=@system-service` | exit=159 (SIGSYS) | append `@mount unshare setns clone clone3` |
| `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX` | exit=1 | add `AF_NETLINK` (bwrap configures loopback) |

`NoNewPrivileges`, `ProtectSystem=strict`, `PrivateDevices`,
`ProtectProc=invisible`, empty `CapabilityBoundingSet`, `MemoryDenyWriteExecute`
are all compatible.

Also: `PrivateTmp=yes` is safe **only** because of `--stream`. With a path-based
scan, clamd sits in the host's `/tmp` and cannot see the private one — a second,
independent breakage.

### Verified working combination

```yaml
attachments:
  scan_cmd: [clamdscan, --no-summary, --stream, "{in}"]
  scan_sockets: [/etc/clamav/clamd.conf]
```

Real HTTP uploads under the corrected hardened unit (`systemd-analyze security`
→ exposure 2.4 OK):

```
POST /api/v1/documents/DOC-0001/_attachments/body
  clean.txt → HTTP 200  (stored)
  eicar.txt → HTTP 422  {"detail":"scan failed: exit status 1"}
```

Hardening independently confirmed still enforced: write `/etc` → BLOCKED, write
`/var/lib/rela` → allowed, `ProtectHome` → `/home` empty.

`SupplementaryGroups=clamav` proved unnecessary on Debian (ships
`LocalSocketMode 666`) but is worth documenting as a conditional for installs
that set `660`.

## Scope

**In scope**

- Fix the ClamAV recipe in `GUIDE-attachment-security.md`: `--stream`, the
`clamd.conf` bind, correct the "no extra configuration" and "`--stream` needs
egress" claims.
- Add a documented, tested hardened systemd unit with the three bwrap-compatible
directives and inline rationale.
- Add `/etc/clamav/clamd.conf` to a default bind list so the stock case is
genuinely zero-config.
- Upgrade the "sandbox unavailable" startup log to WARN when a `scan_cmd` is
configured, and correct the message to name systemd unit directives alongside
the sysctl causes.

**Out of scope**

- Renaming `scan_sockets` → `scan_binds` (the key binds paths, not only sockets).
Worth doing, but it is a config-compat change deserving its own ticket.
- A VM-gated integration test against a real clamd. Every existing test uses a
stub `sh -c` scan command, which is exactly why this survived — but standing up
CI infrastructure for it is separate work.

## Acceptance criteria

1. `GUIDE-attachment-security.md` recipe works verbatim on stock Debian 12 +
`clamav-daemon`: clean upload → 200, EICAR → 422.
2. The guide no longer claims `--stream` requires network egress, and no longer
claims a stock install needs no extra configuration.
3. A hardened systemd unit is shipped in the docs; applying it verbatim yields
`sandbox bubblewrap ...` in the startup log (not "sandbox unavailable").
4. With `/etc/clamav/clamd.conf` in the default binds, a config carrying only
`scan_cmd: [clamdscan, --no-summary, --stream, "{in}"]` scans correctly with no
`scan_sockets` entry.
5. When a `scan_cmd` is configured and no sandbox is available, the startup log
is WARN and its text names the systemd directives as a possible cause.
