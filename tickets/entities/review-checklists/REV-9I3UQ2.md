---
id: REV-9I3UQ2
type: review-checklist
title: 'Review: Pin extractions/setup-just to a commit SHA'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: no Go or frontend change; CI runs the suite on the PR)
- [x] ~~Lint clean (`just lint`)~~ (N/A locally: no Go change; actionlint shows no issue on the changed lines)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go change)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: two-line workflow change; reviewed by the rela-security-reviewer agent instead, no findings in the diff)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3B9X8R (nit, deferred: the installed just binary is not
version-pinned; pre-existing and outside this diff).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: grep shows both uses pinned to `f8a3cce218d9f83db3a2ecd90e41ac3de6cdfd9b # v3.1.0`.
2. PASS: the ref API for tag v3.1.0 returns commit `f8a3cce2...`.
3. Checked by the Archplan CI job on the PR.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: chore)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opened with `gh pr create` and monitored manually, as the task instructs)
