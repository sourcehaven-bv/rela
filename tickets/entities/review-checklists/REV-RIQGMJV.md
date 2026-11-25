---
id: REV-RIQGMJV
type: review-checklist
title: 'Review: Deep-linked list filter on a hyphenated relation name is dropped'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (frontend-only change: `npx vitest run` passes 7098 tests in 496 files)
- [x] Lint clean (`just lint`) (frontend-only change: eslint has no findings in the changed files; `npm run typecheck` passes)
- [x] Comment lint gate clean (`just comment-lint`) (frontend `lint:comments` passes; baseline for filters.ts lowered from 5 to 4)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed; the frontend has no coverage gate)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-1IN1HUO (minor, addressed), RR-BMHT3PN (nit, wont-fix),
RR-5AZVWRS (nit, addressed), RR-W49X58E (nit, addressed), RR-SXET9IM (nit,
deferred). The security reviewer found no issues.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Deep link `/list/all_tickets?filter[belongs-to]=frontend` sends the filter to
the API: PASS (manual check, see IMPL-J1XXN52).
- Metamodel-legal names survive parsing: PASS (vitest).
- `__proto__` and names the metamodel rejects are dropped: PASS (vitest).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after `done`)
