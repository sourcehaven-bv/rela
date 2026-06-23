---
id: BUG-YZ2BK0
type: bug
title: Automation when/validate comparisons are lexicographic, not type-aware
description: 'automation.Engine evaluated when: conditions and validate: checks via filter.MatchValue (string-only), so ordered comparisons (<,<=,>,>=) on integer or date properties compared lexicographically: count>9 was false for count=10 (''10'' < ''9''), and validate: count<9 wrongly passed for count=10. The Engine held no metamodel, so it had no property-type context to compare correctly.'
priority: high
why1: Engine.matchesWhenConditions and evaluateValidation called filter.MatchValue/matchSimple, which compares stringified values lexicographically for ordered operators.
why2: The Engine struct carried no *metamodel.Metamodel, so it couldn't resolve a property's declared type to parse integers/dates before comparing.
why3: matchSimple was introduced as a 'works without full metamodel context' shortcut, accepting lexicographic ordering as good-enough rather than threading the metamodel that NewEngineFromMetamodel already had in hand.
why4: Three parallel comparison evaluators existed (filter.MatchValue string-only, filter.Match type-aware, search.MatchFilters string-only) with no documented boundary, so the automation path used the wrong one.
why5: No test compared an integer property where lexicographic and numeric ordering disagree, so the bug stayed latent.
prevention: 'Engine now holds an optional *metamodel.Metamodel (NewEngineFromMetamodel wires it; SetMetamodel for the rest). when:/validate: comparisons route through the type-aware filter.Match when the property is declared, falling back to string matching only without a metamodel or for undeclared properties. Research RES-6PK0S3 set the boundary: filter.Match = data-filtering, predicate = policy. Regression tests assert count>9 fires numerically for count=10 (and fails without the fix), and that undeclared properties still match via string fallback.'
status: done
---

## Bug

Found in the 2026-06-09 backend review (C2) and scoped by research
**RES-6PK0S3**.

`automation.Engine` evaluated `when:` conditions (`matchesWhenConditions`) and
`validate:` checks (`evaluateValidation`) through `filter.MatchValue` /
`matchSimple` — **string-only** comparison. For ordered operators on
integer/date properties this compares lexicographically:

- `when: count>9` was **false** for `count=10` (`"10" < "9"`), so the automation silently never fired.
- `validate: count<9` **wrongly passed** for `count=10` (`"10" < "9"` is true lexically), emitting no warning.

Root cause: the `Engine` struct held no `*metamodel.Metamodel`, so it had no
property-type context — even though `NewEngineFromMetamodel` already received
the metamodel and discarded it.

## Fix (PR pending)

- `Engine` gains an optional `*metamodel.Metamodel`. `NewEngineFromMetamodel(meta, defs)` wires it; `SetMetamodel` covers the rest. `NewEngine(automations)` stays metamodel-less (string fallback) so existing test engines are unaffected.
- `when:`/`validate:` comparisons route through the type-aware `filter.Match` when the entity type + property are declared (numeric/date-correct), falling back to the string-only `matchSimple` without a metamodel or for an undeclared property (tolerates ad-hoc properties rather than rejecting them).

This is **step 1** of the RES-6PK0S3 sequencing (hybrid: `filter.Match` for
data-filtering, `predicate` for policy). It is an intended behavior change for
any automation rule that relied on lexicographic ordering of integer/date
properties.

## Tests

`internal/automation/typed_comparison_test.go`: `count>9` fires numerically for
`count=10` with the metamodel (and does NOT with a string-only engine —
documents the change); `validate: count<9` warns for `count=10`; an undeclared
property still matches via string fallback. Verified the typed cases **fail
without the fix**.
