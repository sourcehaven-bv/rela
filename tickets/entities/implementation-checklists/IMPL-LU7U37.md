---
id: IMPL-LU7U37
type: implementation-checklist
title: 'Implementation: Converge condition evaluators on predicate: --filter CLI, unify type adapter, migrate automation/validation (TKT-7EJK4 phase 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Scope:** phase 2a — slice 1 (unify type adapter) + slice 2 (filter->predicate
transpiler). Slices 3+4 moved to TKT-J4IR1G.

## Development

- [x] Unit tests written (affordances typed_when_test.go; predicatefns fromfilter_test.go + env/parity tests)
- [x] Integration tests (cross-engine parity: filter.Match vs FromFilter->predicate over an operator x type x record matrix)
- [x] Happy path implemented (shared ScalarType/ScalarTypeForProp; FromFilter)
- [x] Edge cases handled (time.Time vs string dates, empty/missing contract, unsupported ops error, value escaping)
- [x] Error handling (transpile errors for unknown prop / non-int / non-bool / fuzzy-wildcard / list-ordered; fail-closed preserved on the ACL path)

## Test Quality

- [x] Fixture builders (ticket(), policyFromYAML, transpileMeta, bindRaw)
- [x] No hardcoded values where object in scope
- [x] Only values that matter asserted
- [x] ~~Interpolated values from objects~~ (N/A: predicate source strings are the unit)
- [x] Comparisons against source values, not hardcoded

## Manual Verification

- [x] Verified via table-driven + parity tests (go test -race)
- [x] Each AC (1,2,5) verified; AC3/4 (--filter, automation) are phase 2b
- [x] Edge cases verified (injection attempt neutralized; empty/missing; time.Time dates)

**Verification Evidence:** full build + `just arch-lint` clean; `-race` green
across
predicate/predicatefns/affordances/dataentry/statemachine/conditionlint/filter;
golangci-lint 0 issues on changed packages. Only failing package in `go test
./...` is `internal/docscapture` (headless-Chrome `context deadline exceeded`,
unrelated — no predicate/filter/affordances imports; passes in CI).

## Quality

- [x] Follows project patterns (shared helper w/ (Type,ok) omit semantics; consumer keeps field-selection policy; transpiler in the package that owns the func-name constants)
- [x] DRY (single ScalarType source of truth; presentGuard; luaStringLiteral)
- [x] No security issues (RE2 host funcs; injection-safe escaper w/ test; ACL fail-closed preserved; no eval-time I/O)
- [x] No silent failures (transpile + compile errors surfaced)
- [x] No debug code

## Code Review

- [x] `/design-review` run (adversarial, pre-implementation) — 3 critical + 5 significant + minors, all folded/addressed
- [x] All critical addressed (RR-WHMVLW time.Time binder, RR-782ULH date semantic change, RR-TQEHO4 transpiler home/escaping)
- [x] All significant addressed for this scope (RR-IRV2WJ int coercion, RR-NKWJS6 mapping totality, RR-23W88J field-selection). RR-2Y851X/RR-02P03I moved to phase 2b.
