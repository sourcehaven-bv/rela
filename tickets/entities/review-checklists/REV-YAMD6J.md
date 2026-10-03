---
id: REV-YAMD6J
type: review-checklist
title: 'Review: neoq Shutdown stops every listener on the database'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just test` exit 0; `golangci-lint` 0 issues; `just comment-lint` clean;
`just lint-md` 0 issues; `just coverage-check` 81.3% total, thresholds pass.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

No critical findings. Minor findings also fixed: acquire keeps the original
error, its comment is updated, the pending-job loop exits on cancel during
backoff, and the go.mod and CLAUDE.md notes state upstream status accurately.

**Review Responses:** RR-NZ6TL5 RR-D3DZPU RR-4UJUN0 RR-JJ67NA

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: a queue keeps processing after another process closes its queue
  (`TestPostgresQueue_SurvivesAnotherProcessClosing`).

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
