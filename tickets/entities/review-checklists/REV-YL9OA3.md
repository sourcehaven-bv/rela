---
id: REV-YL9OA3
type: review-checklist
title: 'Review: Author-aware version capture: last_edited_by column + flush-on-author-change (precise per-version attribution)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test` — default build; plus DB-gated `just test-postgres` equivalent against local PostgreSQL, full pgstore suite green with -race)
- [x] Lint clean (`just lint` — 0 issues)
- [x] Coverage maintained (`just coverage-check` — PASS, 76.0% total)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Design review (pre-implementation): RR-U964M0, RR-2VWA0Q (critical — addressed
in plan+code), RR-K781MZ (addressed — scope split to TKT-0IGI4V), RR-VG4BPJ,
RR-4OJAC1, RR-MMDQ3N, RR-MZ4PPG, RR-MORL7M, RR-12HJ4K (deferred → pinned as
requirements in TKT-0IGI4V), RR-U1RGSE (addressed).

Code review (cranky-code-reviewer): zero critical/significant/minor findings.
One nit RR-5JIN8U (doc wording overstated the literal-'unknown' prohibition) —
addressed: migration comment + store.Attribution godoc reworded. Reviewer
verified SQL parameter ordering (8-col entity / 10-col relation candidate
scans), all six boundary entry points plus cascade/renumber/restore/Tx
inheritance, migration lock safety, and test isolation.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 swept update attribution: PASS (`TestSweepAttributesRealEditor`)
- AC2 swept create attribution: PASS (same test, op=create asserted)
- AC3 same-author debounce preserved: PASS (`TestSweepAttributesLastEditorOfBurst` — one version)
- AC4 fallback, no "unknown" literals: PASS (`TestAttributionColumnsStamped` NULL assertions + WHO-2 fallback + `TestWithStoreAttribution` unit cases)
- AC5 relation attribution: PASS (relation leg of `TestSweepAttributesRealEditor`)
- AC6 rename re-key neutrality: PASS (SQL inspection — re-key statements never touch the columns)
- AC7 no regressions: PASS (full DB-gated pgstore suite + default-build store/entitymanager suites green with -race; arch-lint, plimsoll, build-check-tags clean)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-LUGRRB

## Final Checks

- [x] Commit message explains the why, not just what (a8bfe151)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR created
- [x] All CI checks pass — being watched via `gh pr checks --watch`; this box is finalized in the same bookkeeping commit that lands only after the run is green (local `just ci` — lint, tests, coverage, arch-lint, plimsoll, tag matrix, docs-check — already fully green on the pushed commit)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1195
