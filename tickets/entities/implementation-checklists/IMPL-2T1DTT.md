---
id: IMPL-2T1DTT
type: implementation-checklist
title: 'Implementation: condition: predicate expressions on list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] Edge cases from planning handled

**Edge cases handled:**

| Case | Behaviour | Pinned by |
|---|---|---|
| Unset property with `~=` | Included (Lua semantics) | `Props_not_equal_or_empty_includes_unset` (all 4 backends) |
| Unset property in a date function | Eval error, surfaced not swallowed | `TestApplyViewCondition` error case |
| List value under an ordered op | Never matches, both backends | `Props_ordered_comparison` |
| Empty value under an ordered op | Never matches | same |
| Caller narrowing vs ACL ceiling | ANDed, never OR-ed | `Narrowing_is_ANDed_not_ORed` |
| No `list_id` | No condition; ACL-scoped superset | `TestViewCondition_DisjunctiveRuleFiltersTheList` |
| Unknown `list_id` | No condition, not an error | `TestViewCondition_UnknownListIDIsUnconstrained` |
| Condition + paging | Count matches filtered population | `TestViewCondition_TotalMatchesTheFilteredPopulation` |
| Condition + pushdown | Pushdown declines explicitly | guard in `listPage`, reasoning in comment |
| Kanban condition | Refused at config load | `TestValidateConfig_KanbanConditionRefusedForNow` |
| Config reload | Recompiled per request, never cached at boot | `App.viewCondition` doc |

## Manual verification

- [x] Feature tested end-to-end
- [x] Each acceptance criterion verified
- [x] Verification evidence documented

**Evidence.** Rather than relying on unit tests agreeing with the
implementation, each store primitive was **mutation-tested** — the change was
deliberately reverted and the test observed to fail with the exact defect it
guards:

- `PropNotEqualOrEmpty` → reverting the SQL to `PropNotEqual` drops the unset
and blank rows (`expected [T-blank T-todo T-unset], actual [T-todo]`).
- ordered ops → removing the `jsonb_typeof <> 'array'` guard makes postgres
include a list row Go excludes (`actual [T-bound T-early T-list]`) — the backend
divergence predicted from Go's `[a b]` vs postgres's `["a", "b"]`.
- `Narrowing` → OR-ing it into the preceding conjunct admits rows the ceiling
excludes (`actual [T-done-mine T-open-mine T-open-yours]`). The wrong append
also no longer compiles: *cannot use `[]store.NarrowBranch` as
`[]store.GraphBranch`*.
- SPA wiring → removing `list_id` fails `EntityList.condition.test.ts`.

Every documented code sample was executed against the live engine. That caught
two errors in my own docs before they shipped: the `is_current_user` sample
needs `CompileWithCurrentUser`, and the quoted startup error was missing its
real `predicate: compile error at line 1:` prefix.

The atlas rule was verified across six fixtures with a pinned clock
(2026-09-13): open card with no date, and `gereed` at 0, 1, 2, 3 and 10 days.

## Quality

- [x] Code follows project patterns
- [x] No silent failures

**Patterns followed.** `conditionlint.CompileViewConditions` mirrors
`CompileNextActions`; the `ViewConditionMatcher`/`Lookup` seams mirror
`ConditionPrefilterer`; `AdaptViewConditions` handles the same typed-nil trap
`NextActionMatchers` documents. arch-lint rejected an earlier wiring that had
`dataentry` importing `conditionlint`/`predicate` — reverted and done the
consumer-side way rather than widening the rule.

**No silent failures** is the governing property of this change, since four of
the five related bugs are the same class:

- a condition that does not compile → **startup error** naming the view
- an evaluation error → **surfaced**, aborting the page, never a dropped row
- a kanban condition → **refused at load** rather than accepted and ignored
- a condition present → pushdown **declines explicitly** rather than silently
returning the unfiltered superset
