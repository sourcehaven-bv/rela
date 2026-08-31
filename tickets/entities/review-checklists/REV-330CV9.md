---
id: REV-330CV9
type: review-checklist
title: 'Review: ClamAV attachment scanning is unusable out of the box: --fdpass, missing clamd.conf bind, and hardened systemd units all break it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

| Gate | Result |
| --- | --- |
| `go build ./...` | clean |
| tests (`cmdexec`, `attachment`, `metamodel`, `dataentry`) | all ok |
| `just lint` | 0 issues (one `honours`→`honors` misspell fixed) |
| `just arch-lint` | OK — no warnings |
| `just comment-lint` | clean, 11479 comments, no unresolvable doc links |
| `just plimsoll` | clean |
| `just coverage-check` | PASS — 78.5% total (33428/42579), package + total thresholds satisfied |

**Comment findings.** `just comment-report` reports nothing in the changed
regions. The two hits in `internal/dataentry/app.go` (lines 138, 549) are
pre-existing and several hundred lines from this diff's edit at ~1040. No
suppressions were added — nothing needed silencing.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviewers ran in parallel: `cranky-code-reviewer` (general quality) and
`rela-security-reviewer` (security invariants). 13 findings total — 12
addressed, 1 wont-fix with evidence. **No open critical or significant.**

**Review Responses:**

*Design review (pre-implementation):* RR-ZH5NSY (significant), RR-U6ZTOM
(significant), RR-9V9B1K (minor), RR-OBIV5G (minor).

*Code review:* RR-8DGI17 (critical), RR-A6MCR5 (critical), RR-JWVDEU
(significant), RR-Y1LLL9 (minor), RR-ZHXSDI (minor, wont-fix).

*Security review:* RR-61YI8G (significant), RR-Q4P93R (significant), RR-5ZFP7T
(minor), RR-9AW9DH (minor).

Both criticals were independently reproduced before fixing, and the fixes
re-verified by re-running the reproduction:

- **RR-8DGI17** — deleting the config bind (the whole point of the ticket) left
  every package green. The test asserted the list *literal*, never that anything
  read it. Now pinned by `TestCmdRunnerBindsScannerDefaults`; the same deletion
  fails with `scanner config "/etc/clamav/clamd.conf" is not bound; clamdscan
  cannot find LocalSocket`.
- **RR-A6MCR5** — `HasConfiguredScan` panicked on a nil metamodel
  (`invalid memory address or nil pointer dereference`). A startup diagnostic
  must not be able to take the server down. Guarded plus a regression test.

**One trap introduced during the fixes and caught before merge.** The
`sandboxReporter` interface refactor (RR-JWVDEU) reintroduced RR-U6ZTOM: a nil
`*CmdRunner` placed in an interface parameter is NOT `== nil`, so the
constructor-failure branch would have been skipped and `SandboxErr()` called on
a nil pointer. The call site now assigns the interface only when construction
succeeded, with a comment naming the trap.

**RR-ZHXSDI rejected with evidence.** The claim that `HasConfiguredScan` should
also walk relation properties does not apply: attachments are entity-only. No
`PropertyTypeFile` reference anywhere is coupled to relations, and
`internal/attachment` contains no relation handling (zero non-test matches), so
there is no relation upload path that could fail closed unwarned.

**Self-review.** `git status` shows only the 11 intended files plus the new test
file. No TODO/FIXME, no debug prints, no `t.Skip` in the diff. The temporary stub
SPA and the throwaway VM harness were both removed. Also fixed nine hand-written
relation files that used `type:` where rela expects `relation:` — all 13
review-response links now resolve.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status** — final verification used the systemd unit extracted
*programmatically from the generated* `docs/attachment-security.md` (not
retyped) and the final binary, in the Debian 12.15 VM:

| AC | Status | Evidence |
| --- | --- | --- |
| 1 — guide recipe works verbatim | **PASS** | clean → 200, EICAR → 422 |
| 2 — false claims removed | **PASS** | `--stream`/egress claim and "no extra configuration" both rewritten; `--fdpass` now carries its explanation |
| 3 — shipped unit yields working sandbox | **PASS** | `sandbox bubblewrap (no network, temp-dir-only writes) + memory/PID/file-size/CPU limits`; `systemd-analyze security` 2.4 OK |
| 4 — zero-config stock case | **PASS** | `grep -c scan_sockets schema.yaml` → 0, still clean 200 / EICAR 422 |
| 5 — WARN + corrected diagnosis | **PASS** | `level=WARN ... EVERY upload to a scanned property will be rejected`; diagnosis leads with the systemd directives |

**Fail-closed re-verified:** with the sandbox deliberately broken, a *clean*
upload still returned 422. Nothing in this change lets an unscannable upload
succeed.

**Recovery re-verified:** restoring the correct unit → warn count 0, clean 200,
EICAR 422.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ECEGAR

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

**Known limitation, recorded deliberately.** No automated test exercises a real
`clamd` under a real systemd unit — every scan test uses a stub `sh -c` command,
which is exactly why this class of bug survived. The VM procedure is written into
the guide so the next person can re-run it, but a CI harness remains out of scope
(planning checklist, Scope/OUT) and is the natural follow-up alongside the
`scan_sockets` → `scan_binds` rename.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A at this point: `/pr`
  gates on the ticket already being `done` and validating clean, so the PR cannot
  exist while this checklist is being completed — see the note below and
  TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
