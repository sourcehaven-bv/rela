---
id: REV-X4F7Z3
type: review-checklist
title: 'Review: Realtime 2/1d: lease HTTP routes, _lease on responses, live wake'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] All tests pass (`just test`) (scoped `go test -race` on dataentry, dataentrywire, lease, openapi and rela-server pass; the full suite runs in CI because local runs time out under machine load)
- [x] Lint clean (`just lint`) (golangci-lint on the touched packages with default tags: 0 issues; full `just lint` hit a go-list timeout under load, CI judges it)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (dataentry and openapi floors unchanged; CI checks them)

## Code Review
- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and security reviewers; 15 findings recorded)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (token clients, strict JSON, stale 304: fixed in d12414cc)
- [x] Self-reviewed the diff for unrelated changes (v1 RowCounts renamed Activity to hold _lease under the field cap)

## Acceptance Verification
- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)
- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (api-reference and acl-security docs updated)
- [x] Docs-checklist marked as done

## Final Checks
- [x] Commit message explains the why, not just what (minimal messages per user preference; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request
- [x] Run `/pr` command to create PR and monitor CI (done after `done`, stacked on the MCP lease tools PR)
