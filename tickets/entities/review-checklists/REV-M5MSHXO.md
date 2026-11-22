---
id: REV-M5MSHXO
type: review-checklist
title: 'Review: Read-only workspace wording and create form'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (focused vitest, vue-tsc, go test ./internal/tenanthost/; the full suite runs in CI)
- [x] Lint clean (eslint and prettier on the changed files)
- [x] ~~Comment lint gate clean~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained~~ (N/A locally: new code has unit tests; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-3WKBBVB, RR-9ZJKYHL)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-3WKBBVB, RR-9ZJKYHL, RR-NQB4SIS, RR-GHKSCSG, RR-VW1DGLT, RR-NQDG6HE, RR-8Q8I7FN, RR-8HBPECQ, RR-UPW0T3G, RR-7G9F5UD

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Workspace wording for a refused write: PASS (TestReadOnly, apiErrors mapping)
- Setting-up screen wording: PASS (en.json and nl.json)
- Create form on a read-only workspace: PASS (DynamicForm.readonly.test.ts)
- Documented: PASS (docs/postgres-backend.md and its guide)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/postgres-backend.md and GUIDE-postgres-backend)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there
