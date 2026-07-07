---
id: REV-B6ISCB
type: review-checklist
title: 'Review: Selection relation not saved when creating an entity in data-entry (edit works)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend 1163 unit tests green; e2e forms + reverse-relations (24) green, including the new BUG-10IPBP create-side test
- [x] Lint clean — `eslint` on changed files rc=0; `vue-tsc` typecheck clean; e2e `tsc` clean
- [x] ~~Coverage maintained~~ (N/A: frontend has no coverage enforcement per frontend/CLAUDE.md; added tests raise effective coverage)

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) — verdict: ship it; fix minimal, edit path untouched, no spurious wipe (two independent guards)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised; one minor + two nits)
- [x] Self-reviewed the diff for unrelated changes — diff is 3 files, all on-topic

**Review Responses:** RR-AYQ1QC (minor, addressed), RR-VQD8FN (nit, addressed),
RR-WY3CO0 (nit, wont-fix with justification)

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist (IMPL-GI3XIJ)

**Acceptance Status:**
- AC "relation selected in create form is saved": **PASS** — e2e creates a feature, picks TASK-002 in the incoming picker, submits; `TASK-002 --implements--> <new feature>` verified from the source side. Manual Puppeteer run captured the create POST body now carrying the incoming edge under the inverse key.
- AC "matches edit-form behavior": **PASS** — edit path unchanged (verified by review + existing edit-side reverse-relations tests still green).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist~~ (N/A: bug fix, no user-facing docs change)
- [x] ~~User-facing documentation~~ (N/A)
- [x] ~~Docs-checklist done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what (pending commit)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending: on develop; needs a feature branch before commit -->
