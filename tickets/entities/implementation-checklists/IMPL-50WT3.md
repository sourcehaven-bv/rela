---
id: IMPL-50WT3
type: implementation-checklist
title: 'Implementation: Extend predicate to a typed superset, then converge the filter/predicate evaluators'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Scope note:** this checklist covers **Phase 1** (extend predicate to a typed
superset). Phase 2 (CLI `--filter`, filter->predicate transpiler,
automation/validation migration) ships as a separate PR with its own
implementation checklist.

## Development

- [x] Unit tests written for new code (typed_coercion_test.go, predicatefns_test.go, env_test.go, parity_missing_test.go)
- [x] Integration tests written (env_test.go exercises metamodel EntityDef -> Env -> compile -> eval end to end)
- [x] Happy path implemented (Int/Date typed comparison, host funcs)
- [x] Edge cases from planning handled (missing/empty parity pinned, int overflow, coercion symmetry, non-integer/malformed-date rejection)
- [x] Error handling in place (compile-time CompileError for bad literals; host-fn errors returned not panicked)

## Test Quality

- [x] Using fixture builders (bind/mustDate closures per test)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: predicate source strings are the unit under test)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually verified via table-driven tests (go test -race)
- [x] Each acceptance criterion verified: AC1 int numeric ordering, AC2 instant-granular dates, AC3 contains, AC4 glob/regex/fuzzy, AC8 no-I/O-at-eval (arch_test green)
- [x] Edge cases manually verified (overflow, LHS-literal symmetry, ==/~= coercion, type mismatches)

**Verification Evidence:** `go test -race ./internal/predicate/
./internal/predicatefns/` green; `golangci-lint` 0 issues; `just arch-lint` OK
(predicate stays `mayDependOn: []`); dependents
(affordances/automation/validation) unaffected; coverage 76%/80% (floor 50%).
AC5/AC6/AC7 (`--filter`, transpile parity, automation migration) are **Phase 2**
— the transpiler target for AC6/AC7 empty-missing parity is pre-verified in
parity_missing_test.go.

## Quality

- [x] Code follows project patterns (consumer-side interface / wiring-site adapter like acl MetamodelView; sealed sum-type extension)
- [x] Checked for DRY opportunities (twoStrings helper; shared twoStr FuncSig; reused filter RE2/trigram rather than reimplementing)
- [x] No security issues introduced (RE2-only matchers documented as load-bearing; no I/O at eval; operator-controlled input)
- [x] No silent failures (errors returned; malformed literals rejected at compile)
- [x] No debug code left behind

## Code Review

- [x] `/code-review` run (cranky-code-reviewer); 0 critical, 3 significant + 4 minor findings
- [x] All significant addressed (RR-O0LM1 int overflow, RR-4189H binder godoc, RR-TBG91 adapter divergence doc)
- [x] Minor: 2 addressed (RR-BNRMU date layouts, RR-YPYTP tz), 2 deferred to Phase 2 with reasons (RR-9V6PF pattern cache, RR-G3Y70 negative literals)
