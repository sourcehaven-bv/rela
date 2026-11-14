---
id: REV-C13O7UA
type: review-checklist
title: 'Review: Scheduled-tasks guide uses peer naming instead of remotes'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: documentation only, no code changed)
- [x] ~~Lint clean (`just lint`)~~ (N/A: documentation only, no Go code changed)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no code comments changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no code changed)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: six-line wording fix in a generated guide and its source)
- [x] ~~All critical review-responses addressed~~ (N/A: no review responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review responses)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: no "peer" left in `docs/scheduled-tasks.md` or its source; the guide names `remotes.yaml` and `remote:<name>` and links to the existing `remote-sync.md`.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore, not an enhancement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: chore, not an enhancement)

**Docs Checklist:** N/A.

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: minimal commit message by project owner preference; the why is in GitHub issue #2166 and this ticket)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR opened with `gh pr create` (#2254)
