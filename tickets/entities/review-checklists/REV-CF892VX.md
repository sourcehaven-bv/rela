---
id: REV-CF892VX
type: review-checklist
title: 'Review: Unset properties from a flows.yaml trigger'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...`)
- [x] Lint clean (golangci-lint on the changed packages; vue-tsc and eslint on the SPA)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no new package; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-GJUSFS0, RR-VCFQVET, RR-51XHWLH, RR-Q9QC4W4,
RR-046PSV8. None critical or significant.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS. `unset:` removes the property on update, patch,
create and cascade create (TestTriggerUnset_*). Load errors for unknown, set and
unset together, required, computed, file and state-machine properties
(TestSetUnset_LoadErrors).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-A17A0WQ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
