---
id: REV-F7STXX
type: review-checklist
title: 'Review: Version sweep selects only rows without a stored hash'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (pre-commit `go test -race ./...` passed; pgstore with the postgres tag and sqlitestore/sqlitedb with the sqlite tag also pass with -race)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages with default, postgres and sqlite tags; arch-lint clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: CI evaluates the floors; tests were added, none removed)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-W29XJT, RR-W592HL (wont-fix: not a defect), RR-3DOK3T, RR-1HE885,
RR-071ZE9, RR-4LC9XP, RR-PA5B23, RR-TTQMQK

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Scan uses the partial index on both backends: PASS (EXPLAIN, see IMPL-5L8UDW)
- Out-of-band version makes the row a candidate again: PASS (VersionWrittenOutsideTheSweep, trigger tests)
- Existing SweepBacklog cases: PASS

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor)
- [x] ~~User-facing documentation updated~~ (N/A: internal; .claude/rules/versioning.md updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist, see TKT-UFV01M)
