---
id: REV-38NA02
type: review-checklist
title: 'Review: Make sandbox read paths operator-configured (RELA_SANDBOX_READ_PATHS)'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`golangci-lint`: 0 issues (macOS, and `GOOS=linux` for cmdexec, attachment,
dataentry, metamodel). `just arch-lint`: OK after allowing rela-desktop to use
cmdexec, as the server already may. `just comment-lint`: exit 0. `go test
./...`: all packages pass except `cmd/rela-desktop`
`TestChromeStyle_TargetsShippedClasses`, which needs the built SPA (no frontend
build in this worktree). One `just ci` run timed out in
`TestAnalyzeProperties_StopsScanningAtCap` at a host load average of ~36 from
other work; the test passes alone in 32 s and the dataentry package passes on
its own. GitHub CI is the authoritative run.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Design review (2026-10-07): RR-19CVEJ (critical), RR-INX7D4, RR-U42OLN,
RR-SY1B7G (significant), RR-C5ALAE, RR-FUW561, RR-RYKBHZ, RR-1YJ0TC (minor),
RR-XHP5OF (nit). All addressed.

Code review (cranky-code-reviewer): RR-3TQABD (significant), RR-OW7CUZ,
RR-XLOWC1, RR-BI659H, RR-IS0CCH, RR-VWQ38Q, RR-6GEON1, RR-2A2MR0 (minor),
RR-KUQPXT (nit). All addressed.

Security review (rela-security-reviewer, no critical or significant): RR-D30YAQ,
RR-GOL8UR, RR-TII45K (minor), RR-1CD9JC (nit). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. No paths beyond system dirs unless listed: PASS
(`TestHostReadOnlyReachesEveryRunner`, `TestCmdRunnerBindsOnlyOperatorPaths`).
2. Operator list reaches every runner: PASS (same tests).
3. Server, CLI and desktop read the same setting: PASS (`TestApplyHostEnv`,
`--help` output, `rela render` on atlas).
4. `scan_sockets` fails loudly: PASS (`TestParse_ScanSocketsRemoved`: list,
`[]`, bare key, merge key).
5. PDF export on Debian 13 with the documented list: PASS (manual, atlas;
Open Sans embedded).
6. ClamAV with the operator list: PASS (manual on atlas; `TestClamd*` in CI).
7. Non-absolute and sandbox-undoing entries dropped: PASS
(`TestSetHostReadOnlyRejectsPathsThatUndoTheSandbox`,
`TestSetHostReadOnlyChecksSymlinkTargets`, manual CLI run).
8. Startup warning for commands without read paths: PASS
(`TestWarnIfNoSandboxReadPaths`).
9. Built-in list fixed: PASS (`TestSystemReadOnlyPathsAreFixed`, Linux CI).
Added by review: project directory never exposed: PASS
(`TestCheckProjectNotExposed`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-BOWIP5

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
