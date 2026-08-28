---
id: REV-FVTZNP
type: review-checklist
title: 'Review: --filter CLI flag + migrate automation/validation onto predicate (TKT-7EJK4 phase 2b)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Scope:** phase 2b — --filter CLI, negative literals, automation/validation
migration onto predicate.

## Automated Checks
- [x] Tests pass (`-race` green: predicate, predicatefns, automation, validation, cli; full `just ci` re-run after fixes)
- [x] Lint clean (golangci-lint 0 issues on changed packages)
- [x] Coverage maintained

## Code Review
- [x] `/code-review` run (cranky-code-reviewer) on the phase-2b diff — found 2 critical + 2 significant + 1 minor + 1 nit
- [x] All critical addressed: RR-G9KT8H (automation untranspilable->filter.Match fallback), RR-FI4DYL (validation untranspilable->filter.MatchAll fallback). Both pinned by new tests.
- [x] All significant addressed: RR-3UR3VH (--where legacy fallback, no hard-error regression), RR-FUD017 (per-eval today() via now func)
- [x] Minor: RR-1NIV6A (coercion-dup doc corrected; full merge = tracked follow-up). Nit: RR-CLYVDL (negative-overflow test added).
- [x] Self-reviewed diff for unrelated changes

**Review Responses:** RR-G9KT8H, RR-FI4DYL (critical, addressed); RR-3UR3VH,
RR-FUD017 (significant, addressed); RR-CLYVDL (nit, addressed); RR-1NIV6A
(minor, deferred w/ reason). Plus the two carried-forward acceptance RRs
RR-2Y851X, RR-02P03I (addressed).

## Acceptance Verification
- [x] AC (--filter flag) PASS — TestApplyListFilters incl. an `or` --where can't express; --where combined + transpiled + deprecation notice
- [x] AC (negative literals) PASS — TestNegativeLiterals + reject corpus
- [x] AC (automation/validation migrated, typed behavior preserved) PASS — existing suites incl. automation-typed-comparison-test green; untranspilable-clause fallback pinned
- [x] AC (no eval-time I/O) PASS — arch_test green

## Documentation
- [x] ~~Docs-checklist~~ (kind=refactor; but user-facing `--filter` flag added) — GUIDE-cli-reference.md updated (--filter + --where deprecation), docs regenerated; CLAUDE.md condition-engine note added. No separate docs-checklist required for a refactor with generated-doc coverage.

## Final Checks
- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs
- [x] Ready for another developer

## Pull Request
- [x] Run `/pr` (this step)
- [x] All CI checks pass (monitored after push)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1315
