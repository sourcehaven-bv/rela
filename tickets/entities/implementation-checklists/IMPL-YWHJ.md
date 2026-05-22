---
id: IMPL-YWHJ
type: implementation-checklist
title: 'Implementation: Predicate language: gopher-lua expression subset for declarative conditions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (compile_test.go, eval_test.go,
      lint_test.go, concurrent_test.go, arch_test.go, fuzz_test.go)
- [x] Integration tests written — the AC1 / AC2 corpora exercise the
      full parse → walk → IR → eval pipeline; AC4 runs five
      worked-use-case scenarios end-to-end with concrete bindings.
- [x] Happy path implemented — all 16 accept-corpus expressions compile
      and evaluate; five end-to-end scenarios produce the expected
      booleans for both true and false binding sets.
- [x] Edge cases from planning handled — BOM strip, leading-return
      reject, multi-statement reject, deep-nesting reject (compile-time
      depth budget), step budget, nil env, missing attr, byte-equal
      string compare incl. null bytes, all six number lexical forms.
- [x] Error handling in place — three typed error types (`ParseError`,
      `CompileError`, `EvalError`), wrapping parser panics via
      `recover()` (RR-S84L); none of the error messages include
      caller-supplied binding values.

## Test Quality

- [x] Using fixture builders or factories for test data — `testEnv(t)`
      helper, `stubFuncs()` overlay, `rec/recAB/rec1num/recNM` helpers.
- [x] No hardcoded values in assertions when object is in scope —
      end-to-end tests compare against bound values, not literals.
- [x] Only specifying values that matter for the test — `stubFuncs`
      defaults the unused funcs; per-case maps override only what
      the case cares about.
- [x] Interpolated values constructed from objects, not hardcoded
      where applicable.
- [x] Property comparisons use original object — the
      `TestProgram_Eval_NilMissingAttr` test reads back what it bound.

## Manual Verification

- [x] Feature manually tested end-to-end — full test suite passes
      under `-race`; fuzz harness ran 715k execs in 10s with no
      crashes/panics.
- [x] Each acceptance criterion verified with the test named in
      planning. AC mapping:
      - AC1 → `TestCompile_AcceptsValidExpressions` (16 files ≥ 15 floor)
      - AC2 → `TestCompile_RejectsDisallowedConstructs` (22 reject files)
      - AC3 → `TestCompile_RejectsUnknownSymbols`, `TestCompile_RejectsNilEnv`
      - AC4 → `TestProgram_Eval_EndToEnd` (5 scenarios × 2 binding sets)
      - AC5 → `TestProgram_Eval_StepBudget`, `TestCompile_RejectsDeeplyNestedExpression`
      - AC6 → `FuzzCompile`, `TestCompile_RecoversParserPanics`
      - AC7 → `TestLintAll_ReportsAllSourceErrors`, `TestLintAll_AllClean`
      - AC8 → `TestPackageImports` + arch-lint config entry
      - AC9 → `TestProgram_Eval_Concurrent` under `-race`
      - AC10 → `TestCompile_NumberLexicalForms`, `TestEval_EqualitySemantics`
      - AC11 → `TestCompile_StripsBOM`, `TestCompile_RejectsLeadingReturn`,
        `TestCompile_LeadingReturnTokenBoundary` (boundary case bonus)
- [x] Edge cases manually verified — leading-return token boundary
      (`returns` identifier) passes a dedicated test.

**Verification Evidence:**

- `go test ./internal/predicate/... -race` — PASS, 1.5s
- `go test ./internal/predicate/... -fuzz=FuzzCompile -fuzztime=10s` —
  PASS, 715k execs, 0 crashes
- `just test` — PASS, full project
- `just lint` — clean, 0 issues
- `just arch-lint` — clean, no boundary violations
- `just coverage-check` — PASS (76.4% total, package floor satisfied)
- `just ci` — PASS, all stages green

## Quality

- [x] Code follows project patterns — package layout matches
      `internal/markdown` style; errors use typed structs with
      Line/Col like other parsers in the tree; consumer-side
      interfaces per CLAUDE.md (env is the consumer-side contract).
- [x] No security issues introduced — allow-list walker with
      default-reject; per-field invariants enforced; compile-time
      depth budget defends stack overflow; per-Eval step budget
      defends CPU exhaustion; no I/O; no panics on adversarial
      input (verified by FuzzCompile).
- [x] No silent failures — every error path returns a typed error;
      `recover()` in Compile converts panics to typed errors instead
      of swallowing.
- [x] No debug code left behind — no fmt.Println, no commented-out
      code, no TODOs.
