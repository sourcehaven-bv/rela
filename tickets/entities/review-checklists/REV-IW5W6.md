---
id: REV-IW5W6
type: review-checklist
title: 'Review: Client-side condition expression engine (parser + evaluator)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `npm run test:run` → 79 files, 1306 passed (35 in `conditions.test.ts`)
- [x] Lint clean — `npm run lint` exits 0; targeted eslint of both new files exits 0 with zero warnings
- [x] ~~Coverage maintained~~ (N/A: frontend has no coverage enforcement per frontend/CLAUDE.md — unit tests run plain; the new module is comprehensively unit-tested regardless)

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer invoked on both files
- [x] All critical review-responses addressed — RR-IROUO (ReDoS cap), RR-P3HL8 (node budget)
- [x] All significant review-responses addressed — RR-ATHC2 (strict decimal coercion)
- [x] Self-reviewed the diff — `git diff --stat` shows only the two new files (+1009 lines), no unrelated changes

**Review Responses:**

Design-review findings (all addressed): RR-8VZSP, RR-9IQBT, RR-8GRLD, RR-P6GVE,
RR-YTKIC, RR-7VKNB, RR-TNMRC. Code-review findings (all addressed):
- RR-IROUO (critical) — ReDoS via `=~`: pattern length cap (MAX_REGEX_LENGTH=200) rejects pathological patterns before running them.
- RR-P3HL8 (critical) — eval-recursion: replaced nesting-only depth counter with a total-node budget (MAX_NODES=500) enforced per emitted node; long flat chains now rejected at parse.
- RR-ATHC2 (significant) — strict-decimal `toNumber` (DECIMAL_RE) rejects hex/binary/whitespace/Infinity.
- RR-7GDOI (minor) — paren double-count eliminated by the node-budget switch.
- RR-KR035 (minor) — non-scalars map to NIL; nil/coercion/eval-guard test gaps closed; eval-time forbidden-key guard documented as belt-and-suspenders.

Reviewer's finding #5 (compareEq bool-branch ordering) was flagged "confirm
intended" not a defect — it is intended (matches the pinned coercion table: bool
literal matches real bool OR 'true'/'false' string, checked before numeric) and
is covered by the bool-coercion test.

## Acceptance Verification

- [x] Each acceptance criterion tested (see AC1–AC7 mapping in IMPL-MIFZB)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All 7 ACs PASS.
1. Grammar parses; precedence pinned by AST-behavior tests (`not a == b`, `a==b and c==d or e`). PASS
2. `program.eval` returns bool; namespaces resolve; `compile` memoized (same-instance test). PASS
3. Coercion/equality table — test per row incl. strict-decimal edge cases. PASS
4. Fail-safe: per-node error → false, short-circuit tests, parse failure → false+warn, nothing throws. PASS
5. Prototype-pollution guard at parse; own-property lookups; non-scalar → nil. PASS
6. Function calls parse but eval rejects (registry deferred). PASS
7. Full operator/precedence/paren/short-circuit/nil/coercion/guard/regex/deferred-fn/complexity coverage. PASS

## Documentation (enhancements only)

- [x] ~~Docs-checklist created~~ (N/A: pure client utility with no user-facing surface yet. The author-facing grammar reference for `visible_when`/`required_when` is written in docs/data-entry.md by the consumer TKT-CHLAJ, where the config keys first appear. The engine's own contract is documented in its module JSDoc.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — grammar docs land with TKT-CHLAJ.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — public API is `compile(expr): Program` / `program.eval(bindings)` / `evaluate(expr, bindings)`, fully documented

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
