---
id: REV-IDU310
type: review-checklist
title: 'Review: Readers could save settings defaults and trigger a git sync'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] ~~Lint clean (`just lint`)~~ (N/A locally: ran golangci-lint on the changed packages with the default tags, 0 issues; the full four-tag `just lint` run is left to the CI Lint job)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Tests: `just coverage-check` runs the full suite with `-race -shuffle=on`. Every
package passed except `cmd/rela-desktop`
(`TestChromeStyle_TargetsShippedClasses`: "no stylesheets in the embedded SPA").
That test needs a built frontend, which this worktree does not have; the change
does not touch `cmd/rela-desktop` or the SPA, and CI builds the frontend first.
`go-test-coverage` on the produced profile: package and total thresholds pass,
total 84.5%.

Lint: `golangci-lint run` on the changed packages (`internal/acl`,
`internal/access`, `internal/dataentry`) reports 0 issues; the default build tag
set covers every changed file. `just comment-lint` reports no unresolvable doc
links. `just arch-lint` reports no warnings.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The cranky-code-reviewer and rela-security-reviewer agents reviewed the PR diff;
the security reviewer re-checked the fixes and found no remaining critical or
significant issue. There were no critical findings. Both significant findings
are addressed (RR-WZYSHB, RR-0SRDNX).

**Review Responses:** RR-WZYSHB, RR-0SRDNX, RR-QAY5D4, RR-RDCAU0, RR-YONYXR,
RR-7EW47J, RR-3YH5TJ, RR-O3HFWZ, RR-1SEZ76, RR-R58UNV, RR-55QTEW, RR-6J6CDG

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

Criteria from the fix plan in BUGA-J6R7BG (bugs have no planning checklist):

- PASS: a principal without `settings:edit` under a policy cannot save the
defaults, palette or logo, or import a theme (`TestSaveSettings_Gate`,
`TestSavePalette_Gate`, `TestThemeLogo_Gate`, `TestThemeImport_Gate`, the
decl-header goldens).
- PASS: a principal without `git:sync` under a policy cannot sync, and
nothing is committed (`TestHandleGitSync_Gate`).
- PASS: read-only refuses both; no `acl.yaml` allows both; unknown, unwired
and broken modes refuse (`TestMode_Decisions`).
- PASS: the git status reports `can_sync` and lists conflict files only to a
syncer (`TestHandleGitStatus_CanSync`, `TestGitStatusResponse_ConflictFiles`).

Evidence is in IMPL-Y3H858.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not an enhancement)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; the operator docs were still updated: the git:sync and settings:edit sections in GUIDE-acl-security)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix, no docs-checklist)

**Docs Checklist:** none (bug)

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: the maintainer asks for minimal commit messages; the why is in BUG-OFW8S0 and the PR body)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

PR #1941 was opened by hand before this ticket existed, following the same steps
as `/pr` (push, open against develop, enable auto-merge); CI is being monitored.
