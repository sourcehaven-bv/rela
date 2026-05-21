---
id: REV-PN0N
type: review-checklist
title: 'Review: Predicate language: gopher-lua expression subset for declarative conditions'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`) — total 76.4%, package
      floor satisfied
- [x] Race-detector clean (`go test ./internal/predicate/... -race`)
- [x] Fuzz harness clean over 10s (`go test -fuzz=FuzzCompile`):
      ~920k execs, 0 panics / crashes / timeouts

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer agent)
- [x] Run architect review (go-architect agent)
- [x] All critical review-responses addressed (none filed in this round)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes (only
      `.go-arch-lint.yml` modified outside the new package, and that
      change is the required component registration + glob fix)

**Review Responses (this round — TKT-2QI1):**

Critical: none.

Significant (all addressed):

- RR-V0OE — per-field AST invariants (AttrGetExpr.Key must be
  *StringExpr; computed table key rejected)
- RR-VI93 — Number lexical-form policy committed to Lua-5.1 single
  type backed by float64
- RR-7VJJ — multi-statement source rejected
- RR-8GOP — leading `return` in source rejected by preprocessor
- RR-XKNO — compile-time depth budget (default 256) defends stack
  overflow on adversarial nested expressions
- RR-BUL2 — *Program concurrency invariant documented + pinned by
  TestProgram_Eval_Concurrent under -race
- RR-UJW6 — Compile signature, nil-env handling
- RR-POA2 — Lua string + nil-vs-false equality semantics
- RR-3XZY — `--[==[` long-bracket comments at level >0 bypass
  leading-return check
- RR-674Z — misleading recover() comment rewritten
- RR-CIGK — evalCall now looks up host fn before evaluating args
- RR-8GPD — Bindings reshaped as a validated builder
- RR-PCLY — Func reshaped as an interface; Eval takes
  context.Context
- RR-93UN — DeclareFunc rejects RecordType / ListType return

Minor (all addressed):

- RR-T4CW — plan-clarity gaps (worked example, public API surface,
  BOM strip, internal/lua in arch-lint, ≥15 file corpus)
- RR-S84L — TestCompile_RecoversParserPanics added
- RR-LQE9 — LintAll → CompileAll, returns programs + issues
- RR-P5DI — sealed-method naming normalized to `sealed<Domain>`
- RR-AJS4 — NewRecord/NewList document ownership transfer
- RR-8VKE — test bundle (hex roundtrip, over-arity table-arg,
  Record.Type pin, fuzz seeds, Issue.Err doc, concurrent-complex)

Nit (addressed):

- RR-S40W — Documentation Planning section cleaned up

## Acceptance Verification

- [x] Each acceptance criterion tested (mapping in IMPL-YWHJ)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC  | Status | Evidence                                              |
|-----|--------|-------------------------------------------------------|
| AC1 | PASS   | `TestCompile_AcceptsValidExpressions` over 16 files   |
| AC2 | PASS   | `TestCompile_RejectsDisallowedConstructs` over 23 files (incl. leading_return_after_long_bracket.lua, over_arity_table_arg.lua, computed_attr_access.lua) |
| AC3 | PASS   | `TestCompile_RejectsUnknownSymbols` + `TestCompile_RejectsNilEnv` |
| AC4 | PASS   | `TestProgram_Eval_EndToEnd` (5 scenarios × 2 binding sets) |
| AC5 | PASS   | `TestProgram_Eval_StepBudget` + `TestCompile_RejectsDeeplyNestedExpression` |
| AC6 | PASS   | `FuzzCompile` (922k execs, 0 panics) + `TestCompile_RecoversParserPanics` |
| AC7 | PASS   | `TestCompileAll_ReportsAllSourceErrors`, `TestCompileAll_AllClean` |
| AC8 | PASS   | `TestPackageImports` + `.go-arch-lint.yml` predicate component canUse=[gopherlua] only |
| AC9 | PASS   | `TestProgram_Eval_Concurrent` + `TestProgram_Eval_Concurrent_Complex` under -race |
| AC10| PASS   | `TestCompile_NumberLexicalForms` + `TestEval_NumberLexicalFormsRoundtrip` + `TestEval_EqualitySemantics` |
| AC11| PASS   | `TestCompile_StripsBOM` + `TestCompile_RejectsLeadingReturn` + `TestCompile_LeadingReturnTokenBoundary` |

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: deferred to ACL integration PR, per
      planning section "Documentation Planning". The predicate
      package is internal with no consumers yet — user-facing docs
      land when the ACL caller wires it up.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A: not created)

## Final Checks

- [x] Commit message explains the why, not just what (pending commit)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — exported API matches
      doc.go lifecycle example; godoc on all public symbols.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
