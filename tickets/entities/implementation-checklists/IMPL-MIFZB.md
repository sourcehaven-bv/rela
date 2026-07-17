---
id: IMPL-MIFZB
type: implementation-checklist
title: 'Implementation: Client-side condition expression engine (parser + evaluator)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `frontend/src/utils/conditions.test.ts` (26 tests)
- [x] ~~Integration tests written~~ (N/A: pure client utility with no collaborators; the wizard consumer TKT-CHLAJ carries the end-to-end e2e coverage. This ticket's "full flow" is compile→eval, exercised directly.)
- [x] Happy path implemented — tokenizer + Pratt parser + tree-walk evaluator; `compile`/`evaluate` public API
- [x] Edge cases from planning handled — nil/unset, coercion table, invalid regex, chaining rejection, depth cap, prototype-pollution guard, deferred function calls
- [x] Error handling in place — fail-safe: parse errors → constant-false program + `console.warn`; per-node eval errors coerce to false locally + warn (never silent, never throws)

## Test Quality

- [x] Using fixture builders or factories for test data — inline `Bindings` literals per case (appropriate for a pure function; no shared fixtures needed)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end — via the test suite (a pure utility; no UI surface yet — that arrives with TKT-CHLAJ)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

All checks run from `frontend/`:
- `npm run test:run -- src/utils/conditions.test.ts` → 26 passed.
- `npm run test:run` (full suite) → 79 files, 1297 passed (no regressions).
- `npm run typecheck` (vue-tsc) → clean.
- `npx eslint src/utils/conditions.ts src/utils/conditions.test.ts` → exit 0.
- `npx prettier --check` → clean (after `--write`).

AC coverage (all 7):
1. Grammar parses; AST-precedence pinned by `not a == b` and `a==b and c==d or e` tests. ✓
2. `program.eval(bindings)` returns bool; `form.`/`entity.`/`current_user.` resolve; `compile` memoizes (same-instance test). ✓
3. Coercion/equality table: one test per row (bool/number/string vs typed+string values, nil, ordered numeric-vs-lexicographic, regex incl. invalid→false+warn). ✓
4. Fail-safe: per-node error → false locally; short-circuit tests on and/or with an erroring operand; parse failure → false+warn; nothing throws. ✓
5. Prototype-pollution guard: `__proto__`/`constructor`/`prototype` field refs rejected at parse; inherited (non-own) props not resolved. ✓
6. Function-call syntax parses but eval rejects (`no such function`) — registry deferred. ✓
7. Full operator/precedence/paren/short-circuit/nil/coercion/guard/regex/deferred-fn coverage. ✓

**A found bug during implementation:** `=~` (regex operator) was initially
omitted from `COMPARE_OPS`, so it tokenized but wasn't accepted in comparison
position — caught by the regex test, fixed.

## Quality

- [x] Code follows project patterns — mirrors `utils/filters.ts` identifier allowlist + prototype-pollution guard; colocated `*.test.ts`; JSDoc module header like other utils
- [x] Checked for DRY opportunities — shared `FORBIDDEN_KEYS`/`IDENT_RE` constants; coercion helpers (`toBool`/`toNumber`) extracted; no premature abstraction
- [x] No security issues introduced — no `eval()`/`Function()`; own-property-only lookups; forbidden-key guard at both parse and eval; depth cap
- [x] No silent failures — every parse/eval failure warns to console; fail-safe coercion is the *documented designed contract*, not a swallowed error
- [x] No debug code left behind
