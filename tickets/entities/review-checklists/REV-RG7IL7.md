---
id: REV-RG7IL7
type: review-checklist
title: 'Review: rename.go upsertEntity/upsertRelation retain pre-BUG-ZWTDH9 create-then-Update-on-ErrConflict fallback (lost-update/clobber)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Tests pass (`just ci` local → exit 0; PR #1132 CI: Test, E2E, Postgres Backend, Fuzz, Frontend, all Cross-Compiles green)
- [x] Lint clean (CI Lint pass; `golangci-lint run` 0 issues; `just arch-lint` OK)
- [x] Coverage maintained (`just coverage-check` → floors PASS)
- [x] All backend builds pass (Cross-Compile darwin/linux/windows × default/postgres all green)

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) — done; 5 findings → RR-PUI4JF, RR-645O6I, RR-LEBF6E, RR-YOFVE3, RR-QWFJ23
- [x] All critical review-responses addressed (none were critical)
- [x] All significant review-responses addressed (RR-PUI4JF, RR-645O6I, RR-LEBF6E — all addressed by the atomic re-route)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-PUI4JF, RR-645O6I, RR-LEBF6E, RR-YOFVE3, RR-QWFJ23 —
all `addressed`

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 "rename never overwrites a target on ID conflict" → PASS (TestRename_TargetExistsDoesNotOverwrite)
- AC2 "entity + all incident relations re-keyed atomically" → PASS (store.RenameEntity; storetest conformance mem/fs/pg)
- AC3 "existing rename behaviour preserved" → PASS (DryRun, applies+rewrites, not-found-typed, ACL fail-closed, entity+relation version capture)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist~~ (N/A: internal bug fix / refactor, no user-facing surface change)
- [x] ~~User-facing documentation~~ (N/A)
- [x] ~~Docs-checklist done~~ (N/A)

## Final Checks

- [x] Commit message explains the why (atomic re-route retires the bug class; references BUG-5QDV6F + #1127)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (net −281 lines; one fewer package)

## Pull Request

- [x] Run `/pr` — PR #1132 created; CI monitored
- [x] All CI checks pass (only the workflow-state gate "Rela Tickets" was red, which clears when this checklist → done and BUG-5QDV6F → done)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1132
