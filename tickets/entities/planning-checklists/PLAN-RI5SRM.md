---
id: PLAN-RI5SRM
type: planning-checklist
title: 'Planning: --filter CLI flag + migrate automation/validation onto predicate (TKT-7EJK4 phase 2b)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/scope/AC — see ticket TKT-J4IR1G. Phase 2b = user-facing half: `--filter` CLI flag + migrate automation/validation onto predicate via the phase-2a `FromFilter` transpiler.

## Research
- [x] RES-6PK0S3 (has-research). Design settled during the phase-2a design-review cycle; the two open RRs (RR-2Y851X cache scoping, RR-02P03I unmigrated-consumers) ARE the design guidance carried forward.
- [x] Re-surveyed current develop (post-2a-merge): automation engine.go (filter.Parse:61, evaluateValidation:329, matchProperty->filter.Match:381/MatchValue:405), validation.go (ParseAll/MatchAll:163-286), cli/list.go (--where:17,91,102) all still on filter; nothing consumes FromFilter yet except phase-2a affordances. Plan holds.

## Approach
- [x] Sequenced, risk-first:
  1. **Negative numeric literals** (RR-G3Y70) — small self-contained predicate walker change; allow UnaryMinusOpExpr over a NumberExpr literal only (fold to negative const), general unary minus stays rejected. Round-trips coerceIntLiteral's ±maxExactIntLiteral guard.
  2. **`--filter` CLI flag** on cli/list.go — predicate-backed (EntityRecordType+Declare+Bind, compile-once). `--where` routes through FromFilter with a one-time deprecation notice at the flag-parse site. Program cache scoped to the metamodel instance (RR-2Y851X). Fold RR-9V6PF (compile-time pattern validation).
  3. **Migrate automation + validation** — transpile legacy filter strings via FromFilter on load, compile-once cache, evaluate per event/entity, entity-only Env. Preserve typed behavior (measure automation-typed-comparison-test). One-time deprecation per rule.
  4. **Golden parity replay + docs**.
- [x] Alternatives rejected: eval-time transpile (re-parses per row); global cache (RR-2Y851X poisoning).

## Security
- [x] Operator-controlled config + operator CLI args (same trust as today's --where). No end-user bodies. FromFilter escaping (phase 2a) + RE2 host funcs carry over. NOTE (develop CLAUDE.md update): "the configuration is not a secret" — permission/config names need no concealment; a 403 naming a missing capability is fine. Nothing here conceals config.

## Test Plan
- [x] Golden replay of real metamodel.yaml automations/validations old(filter) vs new(predicate) — identical verdicts. --filter e2e incl. an `or`. Negative-literal + cache-scoping unit tests.

## Documentation
- [x] cli-reference.md (--filter, --where deprecation), metamodel.md (predicate exprs + legacy compat), CLAUDE.md (predicate=condition engine; filter still backs query-filtering in the RR-02P03I consumers).

## Design Review
- [x] Design settled in the phase-2a adversarial review (this is the deferred user-facing slices of that reviewed plan). No separate /design-review; the open RRs are the guidance. Will /code-review the diff at review time.
