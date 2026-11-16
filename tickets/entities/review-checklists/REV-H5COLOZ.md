---
id: REV-H5COLOZ
type: review-checklist
title: 'Review: remote_mcp config fails open on a null allow and on a decode error'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (internal/project and cmd/rela-server pass locally; the full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on internal/project: 0 issues; full lint runs in CI)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the change only adds tested lines to internal/project; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-OI3K523)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OI3K523, RR-9Z5CU8A, RR-ASA7CZL, RR-DZV8FFZ,
RR-J8BRLDK, RR-7ISAHX6, all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (null allow and misspelled key next to a decode error are refused; TestLoadConfig_RemoteMCP)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS for both cases in sourcehaven-bv/rela#2324, and for
the review's added cases.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (GUIDE-mcp-server lists the refused forms)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
