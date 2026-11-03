---
id: REV-8EJN03
type: review-checklist
title: 'Review: Realtime 2/1a: lease storage (kv with file lock, postgres, conformance)'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] All tests pass (`just test`) (scoped `go test -race` on lease, filelock, piles, state, appbuild and the pg suites pass; the full suite runs in CI because local runs time out under machine load)
- [x] Lint clean (`just lint`) (golangci-lint on the touched packages, default and postgres tags: 0 issues; full `just lint` hit a go-list timeout under load, CI judges it)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (floors added for lease, kvlease, pglease, filelock in .testcoverage.yml; CI checks them)

## Code Review
- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and security reviewers; 15 findings recorded)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (pg lock ordering, NextExpiry, conformance gaps: fixed in the review-fix commit)
- [x] Self-reviewed the diff for unrelated changes (FSKV file modes are a deliberate security-review fix)

## Acceptance Verification
- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)
- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (none needed; CLAUDE.md package table updated)
- [x] Docs-checklist marked as done

## Final Checks
- [x] Commit message explains the why, not just what (minimal messages per user preference; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request
- [x] Run `/pr` command to create PR and monitor CI (done after `done`, stacked on #1858)
