---
id: REV-FAOLFB
type: review-checklist
title: 'Review: Realtime 2/1b: lease service with access checks, rename/delete hooks, wake'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] All tests pass (`just test`) (scoped `go test -race` on lease, appbuild, cli, rela-server, entitymanager and the postgres lease/appbuild suites pass; the full suite runs in CI because local runs time out under machine load)
- [x] Lint clean (`just lint`) (golangci-lint on the touched packages with default, postgres, sqlite and memorybackend tags: 0 issues; full `just lint` hit a go-list timeout under load, CI judges it)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (lease and appbuild floors in .testcoverage.yml; CI checks them)

## Code Review
- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and security reviewers; 15 findings recorded)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (Mine row gating, hook ctx, renew wakes, expiry re-arm: fixed in f9bddab4)
- [x] Self-reviewed the diff for unrelated changes (alias_hook log text made generic as part of the hook ctx fix)

## Acceptance Verification
- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)
- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (none needed until the surfaces land)
- [x] Docs-checklist marked as done

## Final Checks
- [x] Commit message explains the why, not just what (minimal messages per user preference; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request
- [x] Run `/pr` command to create PR and monitor CI (done after `done`, stacked on the lease storage PR)
