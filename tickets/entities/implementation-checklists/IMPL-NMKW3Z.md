---
id: IMPL-NMKW3Z
type: implementation-checklist
title: 'Implementation: ClamAV attachment scanning is unusable out of the box: --fdpass, missing clamd.conf bind, and hardened systemd units all break it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Code changes, all as planned:

| File | Change |
| --- | --- |
| `internal/cmdexec/sandbox_options.go` | `DefaultScannerConfigs` (new list); `Runner.SandboxErr()` |
| `internal/cmdexec/sandbox_linux.go` | `usernsFailure` text now names the systemd directives first |
| `internal/attachment/cmdrunner.go` | binds sockets + configs + operator extras; `SandboxErr()` |
| `internal/metamodel/attachments.go` | `AttachmentPolicy.HasConfiguredScan()` |
| `internal/dataentry/handlers_attachment.go` | `warnIfScanCannotRun` |
| `internal/dataentry/app.go` | calls it outside the `rerr == nil` branch |
| `GUIDE-attachment-security.md` (+ generated `docs/`) | recipe, corrected claims, systemd section |

Design-review findings all implemented: `HasConfiguredScan` is a real method,
not `!HasUnconfiguredScan()` (RR-ZH5NSY); the warn takes `(runner, buildErr)`
and handles a nil runner (RR-U6ZTOM); the opt-out needs no special case
(RR-9V9B1K); the warn sits beside `probeAttachmentCommands` (RR-OBIV5G).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

New tests reuse the existing `fileMetaCmd` builder in
`internal/metamodel/attachments_test.go`; the dataentry test adds local
`captureWarn` / `scanWarnMeta` helpers rather than inlining metamodel literals.

`TestHasConfiguredScan_NotInverseOfUnconfigured` is the regression test for
RR-ZH5NSY: it asserts both methods return true for the same metamodel, so no
future refactor can derive one from the other.

Writing that test caught an error in my own reasoning — my first version claimed
a `scan: off` property makes `HasUnconfiguredScan()` true. It does not; that
method deliberately skips explicit opt-outs. The test was rebuilt around a
property with no scanner at all, which is the real divergence.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Debian 12.15 (bookworm, arm64) VM under lima/vz — no Docker. Stock
`clamav-daemon` + `bubblewrap`, freshclam-updated, `LocalSocket
/var/run/clamav/clamd.ctl`. Real `rela-server` under the real systemd unit.

**AC1 + AC4 — guide recipe verbatim, `scan_sockets` key ABSENT** (`grep -c
scan_sockets schema.yaml` → 0):

```text
POST /api/v1/documents/DOC-0001/_attachments/body
  clean.txt → HTTP 200
  eicar.txt → HTTP 422  {"detail":"scan failed: exit status 1"}
```

Zero-config now genuinely works: before this change the same config failed with
`Can't parse clamd configuration file /etc/clamav/clamd.conf`.

**AC2** — guide re-read; the "works with no extra configuration" claim and the
"`--stream` needs network egress" warning are both gone, replaced with what the
VM actually shows. `--fdpass` now carries an explanation of why it cannot work.

**AC3** — shipped unit installed verbatim:

```text
level=INFO msg="external command confinement"
  detail="sandbox bubblewrap (no network, temp-dir-only writes) + memory/PID/file-size/CPU limits"
→ Overall exposure level for rela-server.service: 2.4 OK :-)
```

**AC5** — reverted the unit to `RestrictNamespaces=yes` to force the failure:

```text
level=WARN msg="attachments: a virus scan is configured but no working sandbox is
available, so EVERY upload to a scanned property will be rejected"
  err="... Under systemd this is usually the UNIT, not the kernel:
       RestrictNamespaces= must allow `user mnt pid net ipc uts cgroup` ..."
```

WARN (not INFO), and the diagnosis leads with the systemd cause instead of
sending the operator to already-correct sysctls.

**Fail-closed preserved** — with the sandbox broken, a CLEAN upload still
returned 422. Nothing here makes an unscannable upload succeed.

**Recovery** — restoring the correct unit: warn count 0, clean 200, eicar 422.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`DefaultScannerConfigs` mirrors `DefaultScannerSockets`; `warnIfScanCannotRun`
mirrors the existing `HasUnconfiguredScan` nudge and sits beside
`probeAttachmentCommands`. `SandboxErr()` is godoc'd as diagnostic-only, so it
does not become the "can I run?" predicate `cmdexec` deliberately refuses.

Security: the only widening is one extra read-only bind of a single config
**file**. `/etc/clamav` as a directory was rejected — it also holds the signature
databases and `freshclam.conf`, which can carry a `DatabaseMirror` proxy
credential. `TestDefaultScannerConfigsAreAbsoluteFiles` fails if a future entry
is a bare directory or a relative path. Fail-closed is unchanged and was
re-verified under a deliberately broken sandbox.

Gates: `go build ./...` clean; `just lint` exit 0; `just arch-lint` OK;
`just comment-lint` clean (11473 comments, no unresolvable doc links);
`just plimsoll` clean; tests green in `cmdexec`, `attachment`, `metamodel`,
`dataentry`. The temporary stub SPA and the throwaway harness used for VM
testing were both removed; `git status` shows only intended files.
