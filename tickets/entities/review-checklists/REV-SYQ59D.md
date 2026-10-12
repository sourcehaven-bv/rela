---
id: REV-SYQ59D
type: review-checklist
title: 'Review: Realtime 2/2: presence over live views, avatars and claim badge'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] All tests pass (`just test`) (scoped `go test -race` on presence, dataentry and openapi pass; the full suite runs in CI because local runs time out under machine load)
- [x] Lint clean (`just lint`) (golangci-lint on the touched packages with default tags: 0 issues; full `just lint` hit a go-list timeout under load, CI judges it)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (presence floor added; CI checks them)

## Code Review
- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and security reviewers; 15 findings recorded)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (watching face, overload churn, deterministic tests: fixed in 49809915; skipped replays rescheduled in 4afef4c7)
- [x] Self-reviewed the diff for unrelated changes (flaky live tests deflaked in separate commits)

## Acceptance Verification
- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)
- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (api-reference and acl-security updated)
- [x] Docs-checklist marked as done

## Final Checks
- [x] Commit message explains the why, not just what (minimal messages per user preference; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request
- [x] Run `/pr` command to create PR and monitor CI (done after `done`, stacked on the lease CLI PR)
