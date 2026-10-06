---
id: REV-25I3NN
type: review-checklist
title: 'Review: E2E fixture and test matrix for faces and worlds'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no Go code changed; the e2e suite was run instead: faces specs 54/54 with --repeat-each=3, smoke of crud/comments/list/entity-detail/relation-cards 56/56)
- [x] Lint clean (`just lint`)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: commentlint scans Go; no Go files changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: Go coverage floors; no Go files changed)

`just lint` was run as `golangci-lint run --allow-parallel-runners` (other
worktrees held the lint lock): 0 issues. `npm run lint` and `npm run typecheck`
in `e2e/` are clean. No frontend code changed.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-QHSLAD, RR-2FGY1O, RR-LQYCV2, RR-DXEWK4, RR-HVCNYA,
RR-CCTOUJ, RR-2IIF4L, RR-G6OZSF, RR-NE7CPG, RR-D2PJUO, RR-323A9S, RR-ST8JFN,
RR-43LPNZ, RR-S6KJRA (addressed); RR-0DVEI4, RR-J30VJF (wont-fix with reason);
RR-1X3ZF3 (addressed nits).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Faced fixture project: PASS (`e2e/tests/faced-project.ts`; the documented node command writes it).
2. `facedTest` / `facedPgTest`: PASS (every faces spec runs on `facedTest`; `facedPgTest` skips without RELA_E2E_DATABASE_URL, as the other postgres specs do).
3. Live specs: PASS (browse, `?world=`, `ID@face`, switcher, edit, create into a face, publish, single-face delete, comments on a face, reader grant in list and detail).
4. Fixme specs: PASS (each names its bug; each body was run with `.fixme` removed and fails today).
5. Existing specs still pass and e2e lint/typecheck clean: PASS.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore, test-only change)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change; e2e/tests/AGENTS.md documents the fixture)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist for a chore)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened after the ticket is done (TKT-UFV01M), with `gh pr create --base faces-intrinsic`)
