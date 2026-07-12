---
id: REV-RG7IL7
type: review-checklist
title: 'Review: rename.go upsertEntity/upsertRelation retain pre-BUG-ZWTDH9 create-then-Update-on-ErrConflict fallback (lost-update/clobber)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Tests pass (`go test -race ./internal/rename/` green; `./internal/entitymanager/` green — full `just test` pending as part of `/pr` CI)
- [x] Lint clean (`golangci-lint run ./internal/rename/` → 0 issues; `go vet` clean; `just arch-lint` → OK)
- [x] Coverage maintained (rename 88.1%; no floor regression)

## Code Review

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent) — pending, user-triggered
- [ ] All critical review-responses addressed
- [ ] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (only rename.go + rename_test.go touched; ticket-tracking files under tickets/)

**Review Responses:** <!-- populated after /code-review -->

## Acceptance Verification

- [x] Each acceptance criterion tested (strict Create never falls through to Update on conflict — TestRename_ConflictDoesNotOverwrite, entity + relation subtests)
- [x] Test evidence documented in implementation checklist (IMPL-5PGAP1: verified the test fails against pre-fix code, passes against the fix)

**Acceptance Status:**
- AC1 "rename never overwrites a racing entity on ID conflict" → PASS (entity subtest: ErrEntityAlreadyExists, 0 UpdateEntity calls)
- AC2 "rename never overwrites a racing relation on key conflict" → PASS (relation subtest: ErrEntityAlreadyExists, 0 UpdateRelation calls)
- AC3 "existing rename behaviour unchanged" → PASS (TestRename_RewritesRelations, TestRename_SelfReferentialCountsTwice, TestRename_TargetExists all green)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist~~ (N/A: internal bug fix, no user-facing surface change — rename CLI/API behaviour is identical except a racing conflict now errors instead of silently clobbering)
- [x] ~~User-facing documentation~~ (N/A)
- [x] ~~Docs-checklist done~~ (N/A)

## Final Checks

- [x] Commit message explains the why (references BUG-5QDV6F + #1127; "strict Create, drop upsert fallback")
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI — pending, user-triggered
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- populated by /pr -->
