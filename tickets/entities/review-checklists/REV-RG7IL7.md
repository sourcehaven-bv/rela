---
id: REV-RG7IL7
type: review-checklist
title: 'Review: rename.go upsertEntity/upsertRelation retain pre-BUG-ZWTDH9 create-then-Update-on-ErrConflict fallback (lost-update/clobber)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Tests pass (`go test ./...` project-wide → 0 failures; `-race` on entitymanager/cli/mcp/store/... green incl storetest + pgstore)
- [x] Lint clean (`golangci-lint run ./internal/entitymanager/` → 0 issues; `go vet` clean; `just arch-lint` → OK after removing the deleted `rename` component)
- [x] Coverage maintained (`just coverage-check` → all floors PASS, total 76.9%)
- [x] All three backend builds pass (default / memorybackend / postgres)

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) — done; 5 findings → RR-PUI4JF, RR-645O6I, RR-LEBF6E, RR-YOFVE3, RR-QWFJ23
- [x] All critical review-responses addressed (none were critical)
- [x] All significant review-responses addressed (RR-PUI4JF, RR-645O6I, RR-LEBF6E — all `addressed` by the atomic re-route)
- [x] Self-reviewed the diff for unrelated changes (only rename adapter, its test, arch-lint config, and the deleted package)

**Review Responses:** RR-PUI4JF (significant, addressed), RR-645O6I
(significant, addressed), RR-LEBF6E (significant, addressed), RR-YOFVE3 (minor,
addressed), RR-QWFJ23 (minor, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 "rename never overwrites a target on ID conflict" → PASS (TestRename_TargetExistsDoesNotOverwrite: ErrEntityAlreadyExists, 0 creates/updates/deletes, target title intact)
- AC2 "entity + all incident relations re-keyed atomically" → PASS (routed through store.RenameEntity; storetest conformance across mem/fs/pg)
- AC3 "existing rename behaviour preserved" → PASS (DryRun-no-writes, applies+rewrites, not-found-typed, ACL fail-closed, version prev-id, audit before/after all green)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist~~ (N/A: internal bug fix / refactor, no user-facing surface change — rename CLI/API behaviour identical, a target-ID conflict now errors instead of clobbering)
- [x] ~~User-facing documentation~~ (N/A)
- [x] ~~Docs-checklist done~~ (N/A)

## Final Checks

- [x] Commit message explains the why (atomic re-route, retires the bug class; references BUG-5QDV6F + #1127)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (net −281 lines; one fewer package to reason about)

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI — pending, user-triggered
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- populated by /pr -->
