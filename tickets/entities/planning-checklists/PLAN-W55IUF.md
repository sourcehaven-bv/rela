---
id: PLAN-W55IUF
type: planning-checklist
title: 'Planning: ClamAV attachment scanning is unusable out of the box: --fdpass, missing clamd.conf bind, and hardened systemd units all break it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:

1. Fix the ClamAV recipe in `docs-project/entities/guides/GUIDE-attachment-security.md`
— `--stream` instead of `--fdpass`, correct the "stock install works with no
extra configuration" and "`--stream` needs network egress" claims.
2. Ship a tested hardened systemd unit in the guide, with the three
bwrap-compatible directives and inline rationale.
3. Add `cmdexec.DefaultScannerConfigs` (new list) so the stock Debian case needs
no `scan_sockets` entry at all.
4. Upgrade the startup log to WARN when a `scan_cmd` is configured AND no sandbox
is available, and correct the message to name systemd unit directives alongside
the existing sysctl causes.

OUT:

- Renaming `scan_sockets` → `scan_binds`. The key binds arbitrary paths, not only
sockets, and the misleading name is what made a legitimate config look like a
hack. Real, but it is a config-compat change and deserves its own ticket.
- A VM-gated CI integration test against a live clamd. Every existing test uses a
stub `sh -c` scan command, which is exactly why this survived — but standing up
that infrastructure is separate work. Manual VM verification is the evidence
here.
- Changing `--fdpass` support in rela. It is a ClamAV/bwrap interaction, not
something rela can fix; the correct response is to stop recommending it.

**Acceptance Criteria:**

1. **Guide recipe works verbatim on stock Debian 12 + `clamav-daemon`.**
Test: Debian 12 VM, `apt install clamav-daemon bubblewrap`, project whose
schema.yaml carries the guide's recipe verbatim; POST clean.txt → 200, POST
eicar.txt → 422.
2. **Guide no longer carries the two false claims.**
Test: grep the guide for "no extra configuration" and the `--stream`/egress
warning; both absent or rewritten. Reviewed by reading.
3. **Shipped systemd unit yields a working sandbox.**
Test: install the unit verbatim in the VM, `systemctl start rela-server`, assert
the startup log says `sandbox bubblewrap ...` and NOT `sandbox unavailable`;
assert `systemd-analyze security` stays ≤ 3.0.
4. **Zero-config stock case.**
Test: schema.yaml with ONLY `scan_cmd: [clamdscan, --no-summary, --stream,
"{in}"]` and NO `scan_sockets:` key; clean → 200, eicar → 422 in the VM. Unit
test: `DefaultScannerConfigs` is non-empty and every entry is absolute.
5. **WARN when configured-but-unsandboxed.**
Test: unit test asserting the WARN fires when a scan_cmd exists and the sandbox
is unavailable, and does NOT fire when the sandbox works or no scan_cmd is
configured. Message text names systemd directives.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the investigation was empirical (VM reproduction), and
its findings are recorded in the ticket body rather than a separate RES entity.
No design alternatives to survey: the failures were diagnosed to root cause with
direct evidence.

**Existing Solutions:**

- **No library involved.** These are ClamAV/bubblewrap/systemd interaction
defects, diagnosed by bisection in a real VM.
- **Prior art in-repo — the pattern to copy:** `cmdexec.DefaultScannerSockets`
(`internal/cmdexec/sandbox_options.go:20`) is exactly the shape needed for the
new config list: a package-level slice of well-known absolute paths, appended to
the caller's extras in `attachment.NewCmdRunner`
(`internal/attachment/cmdrunner.go:50`), bound with `--ro-bind-try` so missing
paths are skipped harmlessly. `DefaultScannerConfigs` mirrors it exactly.
- **Prior art for the WARN:** `NewApp` already emits a scan-related nudge via
`HasUnconfiguredScan()` (`internal/dataentry/app.go:1106-1112`) — the "no
scanner configured" case. The new warning is its mirror image ("scanner
configured but unusable") and belongs beside it, using the same `slog.Warn(...,
"docs", ...)` shape.
- **Prior art for the diagnosis text:** `usernsFailure`
(`internal/cmdexec/sandbox_linux.go:59`) already special-cases a known cause
class and rewrites the error into operator-actionable prose. Extending that
string is the established mechanism; no new machinery.
- **Related tickets:** BUG-2J30F3 and BUG-OJNWVK both concern bubblewrap being
absent in CI — same "sandbox missing ⇒ commands refuse" failure mode, from the
environment side. Confirms the failure mode is recurrent and worth a louder log.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*1. `DefaultScannerConfigs` (AC4).* Add to `internal/cmdexec/sandbox_options.go`
beside `DefaultScannerSockets`:

```go
// DefaultScannerConfigs are well-known scanner CONFIG files. clamdscan parses
// clamd.conf at startup to find LocalSocket, before it connects — so binding
// the socket alone is not enough. readOnlyPaths deliberately excludes /etc
// wholesale, which is why these need naming individually.
var DefaultScannerConfigs = []string{
    "/etc/clamav/clamd.conf",           // Debian/Ubuntu
    "/usr/local/etc/clamav/clamd.conf", // source builds, BSD
}
```

`attachment.NewCmdRunner` appends it alongside the sockets. Deliberately a
SEPARATE list from `DefaultScannerSockets`: a config file is not a socket, and
conflating them is the naming confusion that made `scan_sockets` look like a
hack in the first place. Binding is per-FILE, never the `/etc/clamav` directory
— that directory also holds signature DBs and `freshclam.conf`, which can carry
a `DatabaseMirror` proxy credential.

*2. Startup WARN (AC5).* Emitted from `internal/dataentry/app.go` at the same
startup point as the existing `HasUnconfiguredScan()` nudge, and placed beside
the `probeAttachmentCommands(meta, runner)` call — that function
(`handlers_attachment.go:264-289`) already walks the global and per-property
scan commands for its missing-binary warning, so the new diagnostic belongs with
it rather than introducing a third traversal of `meta.Entities` (**RR-OBIV5G**).

*Condition.* Fire iff a scan is genuinely configured AND commands cannot run:

```text
scanConfigured  ∧  (runner == nil  ∨  runner.SandboxErr() != nil)
```

Two corrections over the first draft:

- `scanConfigured` must NOT be `!HasUnconfiguredScan()` (**RR-ZH5NSY**). That
  predicate is not the inverse: with two file properties — one carrying a
  `scan_cmd`, one `scan: off` — `HasUnconfiguredScan()` is false while a scan IS
  configured, suppressing the warning exactly when it matters. Add
  `AttachmentPolicy.HasConfiguredScan()` to `internal/metamodel/attachments.go`,
  defined as "∃ a file property where `ScanCommandFor(prop)` is non-nil", which
  reuses the existing resolver and so honours `scan: off` and the per-property
  override for free.
- The `runner == nil` disjunct is required, not defensive (**RR-U6ZTOM**).
  `app.go:1033-1043` leaves `attachmentRunner` nil when `NewCmdRunner` fails, and
  uploads then degrade to MIME-validation-only — one of the two states worth
  warning about. Calling `SandboxErr()` unconditionally would nil-panic at boot
  in precisely that case.

The two causes are distinct (constructor failure vs. an unusable host sandbox),
so the message must not attribute the systemd cause to a nil runner; log the
underlying reason rather than a guess.

*Accessor.* `Describe()` is documented "never branch on this string", so this
must not parse it. Add `cmdexec.Runner.SandboxErr() error` returning the stored
`sandboxErr`, surfaced as `attachment.CmdRunner.SandboxErr()`. Godoc it as
diagnostic-only: it reports *posture*, it is not the forbidden "can I run?"
pre-flight gate, and `Run` remains the only execution path.

*3. Corrected diagnosis (AC5).* Extend the `usernsFailure` branch in
`sandbox_linux.go` to also name the systemd causes, since under a hardened unit
the current sysctl-only text sends the operator to edit already-correct sysctls:

> ...or a systemd unit restricting namespaces (RestrictNamespaces=, needs
> `user mnt pid net ipc uts cgroup`), syscalls (SystemCallFilter=, needs
> `@mount unshare setns clone clone3`), or address families
> (RestrictAddressFamilies=, needs AF_NETLINK)

*4. Guide rewrite (AC1–AC3).* In `GUIDE-attachment-security.md`: swap the recipe
to `--stream`; delete the "works with no extra configuration" claim and replace
with what is now true (sockets AND clamd.conf bound by default); rewrite the
`--stream`/egress paragraph — `--stream` over a `LocalSocket` needs no egress,
only a TCP `clamd` on another host does; add a `--fdpass` note explaining it
cannot work under bwrap; add a "Running under systemd" section with the full
unit and the three-directive table.

**Alternatives considered:**

- *Append clamd.conf to `DefaultScannerSockets`* — smallest diff, rejected: the
name would then lie, reinforcing the exact confusion this ticket documents.
- *Add `/etc/clamav` to `readOnlyPaths`* — rejected: applies to EVERY sandboxed
command (pandoc, exiftool), exposing signature DBs and a possible proxy
credential in `freshclam.conf` for no benefit to those tools.
- *Parse `Describe()` for availability* — rejected: its godoc forbids branching
on it. A typed accessor is correct.
- *Make rela auto-detect and rewrite `--fdpass` → `--stream`* — rejected:
`scan_cmd` is an operator-authored argv, and silently rewriting it would be
surprising and would mask a misconfiguration the docs should teach.

**Files to modify:**

- `internal/cmdexec/sandbox_options.go` — add `DefaultScannerConfigs`.
- `internal/cmdexec/sandbox_linux.go` — extend `usernsFailure` diagnosis text.
- `internal/cmdexec/cmdexec.go` — add `Runner.SandboxErr()` accessor.
- `internal/attachment/cmdrunner.go` — bind configs; expose `SandboxErr()`.
- `internal/metamodel/attachments.go` — add `AttachmentPolicy.HasConfiguredScan()`
  (added per **RR-ZH5NSY**; omitted from the first draft).
- `internal/dataentry/app.go` — WARN when scan configured + sandbox unavailable.
- `internal/dataentry/handlers_attachment.go` — emit the WARN beside
  `probeAttachmentCommands` (**RR-OBIV5G**).
- `docs-project/entities/guides/GUIDE-attachment-security.md` — recipe, claims,
systemd section.
- Tests: `internal/cmdexec/sandbox_options_test.go`,
`internal/attachment/cmdrunner_test.go`, `internal/dataentry/app_test.go` (or
the nearest existing equivalents).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `attachments.scan_cmd` / `scan_sockets` — operator-authored config, already
trusted at the operator-shell boundary. `scan_sockets` entries are validated
absolute by `WithExtraReadOnly` (non-absolute warned + skipped). The new
`DefaultScannerConfigs` is a compile-time constant list, so no new input.
- Uploaded bytes — untrusted, unchanged by this work. Still MIME-sniffed and
scanned before storage.
- No new user input, no new network surface, no new endpoint.

**Security-Sensitive Operations:**

- **Widening the sandbox mount view (the one real risk).** Adding a default bind
grants every sandboxed command read access to that path. Mitigated by binding a
single FILE (`clamd.conf`) not the directory, and by `clamd.conf` containing no
credential — unlike its sibling `freshclam.conf`, which is exactly why the
directory bind is rejected. `--ro-bind-try` keeps a missing path harmless.
- **Fail-closed is preserved.** Nothing here makes an unscannable upload succeed.
The WARN is diagnostic only; it must not become a bypass.
- **The systemd unit must stay genuinely hardened.** The three relaxations are
narrow (an allowlist, specific syscalls, one address family) and were verified
to leave `ProtectSystem=strict`, `ProtectHome`, and the empty
`CapabilityBoundingSet` enforcing — `systemd-analyze security` 2.4 OK. AC3 pins
the score so a future edit cannot quietly gut the unit.
- **No sensitive data in errors.** The diagnosis text names directive names, not
host paths or config contents.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | VM: guide recipe verbatim → clean 200 / eicar 422 |
| 2 | Read-through of the guide; grep for the removed claims |
| 3 | VM: install unit verbatim → log says `sandbox bubblewrap`; `systemd-analyze security` ≤ 3.0 |
| 4 | Unit: `DefaultScannerConfigs` non-empty, all absolute. VM: schema with NO `scan_sockets` → clean 200 / eicar 422 |
| 5 | Unit: WARN fires iff (scan configured ∧ sandbox unavailable); message names systemd directives |

**Edge Cases:**

- `clamd.conf` absent (clamav not installed) → `--ro-bind-try` skips it; no error.
- Both a default conf path AND an operator `scan_sockets` entry → both bound, no
duplicate-bind failure (bwrap tolerates; assert no error).
- `scan: off` on every file property + unusable sandbox → new WARN must NOT fire
(nothing will be scanned, so nothing will be rejected).
- **Mixed properties: one `scan_cmd`, one `scan: off`, sandbox unusable → WARN
MUST fire.** This is the case that `!HasUnconfiguredScan()` gets wrong
(**RR-ZH5NSY**); it is the regression test pinning `HasConfiguredScan()`.
- **`NewCmdRunner` fails ⇒ `attachmentRunner == nil` + scan configured → WARN
fires, no panic** (**RR-U6ZTOM**). Asserts the nil disjunct and that the message
does not blame systemd for a constructor failure.
- Sandbox explicitly disabled via `RELA_UNCONFINED_COMMANDS=1` → must NOT fire
the new WARN; that is a deliberate operator choice, already logged separately.
No suppression logic needed — `New` takes the opt-out branch first and leaves
`sandboxErr` nil, so the condition already excludes it (**RR-9V9B1K**). Test
kept to pin that interaction.
- Non-Linux host (macOS dev) → `DefaultScannerConfigs` paths simply don't exist;
skipped. Linux-only assertions guarded by `runtime.GOOS`.
- Duplicate path appearing in both defaults and operator config → harmless.

**Negative Tests:**

- Sandbox unavailable + scan configured → upload MUST still be rejected (422),
never silently unconfined. Pins fail-closed.
- A relative path in `scan_sockets` → still warned and skipped
(existing `TestWithExtraReadOnlySkipsNonAbsolute` must keep passing).
- The guide's OLD `--fdpass` recipe must still fail in the VM — confirming the
documented reason is the real one and the new recipe is not cargo-culted.

**Integration test approach:** the decisive evidence is the Debian VM (lima/vz,
no Docker): real `rela-server` under the real systemd unit, real `clamd`, real
HTTP uploads. Go-level tests cover the list contents, the accessor, and the WARN
condition; they cannot cover the bwrap/systemd interaction, which is precisely
the gap that let this ship.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Distro variance.** Verified on Debian 12 only. RHEL/Alpine put `clamd.conf`
elsewhere and may name the socket differently. *Mitigation:* include the known
alternate path in `DefaultScannerConfigs`, keep `scan_sockets` as the documented
escape hatch, and state in the guide which distro was verified rather than
implying universality.
- **systemd version variance.** Directive semantics (notably `@mount` contents)
differ across systemd releases. *Mitigation:* record the tested version in the
guide; the unit degrades to fail-closed (not unconfined) if a directive is
stricter elsewhere.
- **The new WARN could become noise** if it fires when nothing would be scanned.
*Mitigation:* explicit edge-case tests for `scan: off` and for the deliberate
unconfined opt-out.
- **`SandboxErr()` could be misused as a pre-flight gate**, which the package
deliberately forbids. *Mitigation:* godoc it as diagnostic-only, mirroring the
existing `Describe()` warning; `Run` stays the single execution path.
- **Docs drift again.** The root cause is that no test exercises a real clamd.
*Mitigation:* out of scope here, but the VM procedure is written into the guide
so the next person can re-run it; a CI harness is the follow-up.

**Effort:** m — small, well-located code changes plus a substantial doc rewrite
and VM re-verification.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-attachment-security.md` — the primary
deliverable: recipe, the two corrected claims, `--fdpass` explanation, and a new
"Running under systemd" section with the full hardened unit.
- [x] Godoc — `DefaultScannerConfigs`, `SandboxErr()`, and the amended
`usernsFailure` message are all operator-facing text.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change — `scan_sockets` keeps its
current meaning; the rename is out of scope)
- [x] ~~CLAUDE.md~~ (N/A: no new pattern or convention)
- [x] ~~README.md~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Finding | Resolution |
| --- | --- | --- | --- |
| RR-ZH5NSY | significant | Warn-condition underspecified; `!HasUnconfiguredScan()` is not the inverse and false-negatives on a mixed `scan_cmd` / `scan: off` metamodel | Plan now specifies a new `AttachmentPolicy.HasConfiguredScan()` built on `ScanCommandFor`; `internal/metamodel/attachments.go` added to Files to modify; regression edge case added |
| RR-U6ZTOM | significant | `runner.SandboxErr()` would nil-panic when `NewCmdRunner` fails — one of the two states the WARN targets | Condition changed to `runner == nil ∨ SandboxErr() != nil`; message must not attribute the systemd cause to a constructor failure; edge case added |
| RR-9V9B1K | minor | Plan implied `RELA_UNCONFINED_COMMANDS=1` needs explicit suppression | Documented that `New`'s opt-out branch leaves `sandboxErr` nil, so the condition excludes it by construction; test retained |
| RR-OBIV5G | minor | New WARN would duplicate a third walk of `meta.Entities` | Placed beside `probeAttachmentCommands`, which already performs that traversal |

No critical findings. All four are addressed in the plan above; implementation is
mechanical from here.
