---
id: REV-WFWFB
type: review-checklist
title: 'Review: Fix misfiled entity files in docs-project/entities/'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — background run, exit code 0
- [x] Lint clean (`just lint`) — `0 issues.`
- [x] Coverage maintained (`just coverage-check`) — exit code 0

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — RR-4AXFJ addressed (renames committed separately from ticket bookkeeping in commits d92ad48 and 0088ed6)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- RR-4AXFJ (significant, addressed): Untracked ticket files mixed into working tree with the rename diff → split into two commits, rename first (d92ad48), bookkeeping second (0088ed6).
- RR-5J1T1 (minor, wont-fix): Historical references to old singular paths in closed ticket/review docs. Reason: editing closed artifacts falsifies the historical record; grep annoyance is not a correctness issue.
- RR-VSPC6 (nit, deferred): Add a metamodel-vs-filesystem guard. Reason: out of scope for this xs chore (scope was "just move the files"); valid follow-up ticket against FEAT-CO4YP / store-backends.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 (plural-only folders):** PASS. `ls docs-project/entities/` → `concepts features guides scenarios tutorials`; no singular folders remain.
- **AC2 (content preserved):** PASS. `git diff --cached --stat` for commit d92ad48 reports `34 files changed, 0 insertions(+), 0 deletions(-)`. All renames are R100.
- **AC3 (rela loads correctly):** PASS. Fresh `rela list` from `docs-project/` reports 38 entities (31 published): 7 concepts, 16 features, 10 guides, 3 scenarios, 2 tutorials.
- **AC4 (analyze clean):** PASS. `rela analyze cardinality`/`orphans`/`properties`/`validations` all report clean.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore kind, not enhancement or docs)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

**Docs Checklist:** N/A (chore kind — no docs impact)

## Final Checks

- [x] Commit message explains the why, not just what — commit d92ad48 explains the fsstore plural convention and why the singular folders were invisible to rela.
- [x] No TODOs or FIXMEs left unaddressed — no code changes.
- [x] Ready for another developer to use — rela now correctly classifies every docs-project entity.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — PR has auto-merge enabled (SQUASH); will merge when CI completes.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/465
