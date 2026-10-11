---
id: REV-S55ZKT
type: review-checklist
title: 'Review: Collapsed background-job trigger resets the hop count'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: full `go test -race ./...` on the stack top, and CI on #1795 (29 checks green).
- [x] Lint clean: `golangci-lint run ./internal/appbuild/` reports 0 issues; CI Lint green.
- [x] Comment lint gate clean: CI Comment lint green on #1795.
- [x] Coverage maintained: CI coverage check green on #1795.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: none found)
- [x] All significant review-responses addressed: RR-XZDSLW.
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-XZDSLW (significant, addressed), RR-ETEYF8 (minor,
addressed), RR-ZSR6YX (minor, wont-fix), RR-XL5A43 (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested: a chain of jobs that trigger each
other runs exactly hops 1..8, under stress.
- [x] Test evidence documented in implementation checklist (IMPL-HQSLXM)

**Acceptance Status:** PASS. 1,800 stress runs, 0 failures; 5 in 1,200 before.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: lands in the existing PR #1795)
