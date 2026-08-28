---
id: IMPL-8UOHYC
type: implementation-checklist
title: 'Implementation: --filter CLI flag + migrate automation/validation onto predicate (TKT-7EJK4 phase 2b)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Scope:** phase 2b — negative literals, --filter CLI, automation/validation
migration onto predicate. Slices 1+2 (adapter+transpiler) landed in phase 2a
(TKT-MOCIED / PR #1305).

## Development
- [x] Unit tests (predicate negative-literals; predicatefns Evaluator/bind; cli applyListFilters; automation typed-comparison + untranspilable-fallback; validation untranspilable-fallback)
- [x] Integration (automation/validation suites exercise the full event/rule path through predicate)
- [x] Happy path (--filter, negative literals, migrated when/validate/When/Then)
- [x] Edge cases (untranspilable clause -> filter fallback in all 3 consumers; empty/missing; per-eval today())
- [x] Error handling (bad --filter -> clear CLI error; transpile error -> legacy fallback, not a verdict flip)

## Test Quality
- [x] Fixture builders (newAutomation, buildEntity, filterTestMeta, newMockWorkspace)
- [x] No hardcoded values where object in scope
- [x] Only values that matter asserted
- [x] Comparisons against source values

## Manual Verification
- [x] Verified via table-driven + suite tests (go test -race)
- [x] Each AC verified (--filter incl. or; negative literals; migration preserves typed behavior incl. automation-typed-comparison-test; no eval-time I/O)

**Verification Evidence:** `-race` green across
predicate/predicatefns/automation/validation/cli/entitymanager/autocascade;
golangci-lint 0 issues (incl. contextcheck fixed by threading ctx through
automation Process); arch-lint clean; docs regenerated. `just ci` `test` stage
fails ONLY on internal/docscapture (TestStandUp_ServesSeededEntity + capture
tests, `data-entry.yaml line 44 export/[]string unmarshal`) which is
PRE-EXISTING on clean develop and untouched by this branch.

## Quality
- [x] Follows patterns (shared Evaluator/binder; consumer-side; ctx threaded)
- [x] DRY (Evaluator + EntityRecord shared across CLI/automation/validation)
- [x] No security issues (RE2 host funcs; injection-safe escaper; no eval I/O; operator-controlled input)
- [x] No silent failures (untranspilable -> legacy fallback preserves exact verdict, pinned by tests)
- [x] No debug code

## Code Review
- [x] `/code-review` run (cranky) — 2 critical + 2 significant + 1 minor + 1 nit
- [x] All critical addressed (RR-G9KT8H, RR-FI4DYL — untranspilable-clause verdict flips, fixed + tested)
- [x] All significant addressed (RR-3UR3VH --where fallback, RR-FUD017 per-eval today())
- [x] Minor RR-1NIV6A deferred (coercion-dup, doc corrected, full merge tracked); nit RR-CLYVDL addressed
