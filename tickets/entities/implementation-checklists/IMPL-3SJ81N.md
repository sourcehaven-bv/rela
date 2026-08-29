---
id: IMPL-3SJ81N
type: implementation-checklist
title: 'Implementation: Multi-enum list filter renders a native listbox and never matches any row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Changes:**

Backend (`internal/dataentry`):

- `propertyElements(prop any) []string` (`helpers.go`) — renders a property as
the elements a filter compares against: one entry for a scalar, one per item for
a list. Non-string list items keep the `%v` form scalars already used, so a
`[]any{1,2}` behaves like the scalars `1` and `2` rather than being dropped.
- `anyElement(prop, match)` — ANY-element predicate. `ne` is implemented as its
negation, so "no element matches" is the exact complement of "some element
matches" and the two cannot drift.
- `matchesAnyCSV(el, vals)` — the comma-list membership test, shared by `in`
and `ne`. These had two copies of the same trim-and-compare loop; `ne` is
*defined* as "not in the `in`-set", which only holds if both read the set the
same way.
- `applyV1Filters` (`api_v1.go`) — the `propStr := fmt.Sprintf("%v", propVal)`
flattening is gone from `eq`/`ne`/`contains`/`in`.

Frontend (`FilterBar.vue`):

- `multi-select` renders `TagSelect` instead of `<select multiple>`, reusing the
`optionLabels` map the component already builds.
- `handleMultiSelectChange` takes `string[]` from the component (not a DOM
`Event`) and sets `op: 'in'`; clearing the selection drops the operator so the
filter disappears rather than emitting an empty `in`.
- Dead `select[multiple]` CSS removed (both the base rule and the mobile
override).

**Ordered operators deliberately unchanged.** `lt/lte/gt/gte` keep the
whole-value form. What it means to order a list against a bound is a design
question, not a defect; inventing semantics for it inside a bugfix is how the
next inconsistency gets created. Commented at the call site.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Go tests are table-driven with `t.Run` subtests and a per-case fresh app
(`newApp(t)`) so one case's filtering cannot leak into the next. IDs are consts
referenced in both seed and expectation rather than repeated literals.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built a minimal project reproducing the reported scenario (`kader` entity with a
`gebieden` list-of-enum + a filter control), served it with `rela-server`, and
drove the real SPA in a browser.

*API, direct (`curl`, 3 entities):*

| Query | Result | Correct |
| --- | --- | --- |
| `filter[gebieden]=Informatiebeveiliging` | `[KDR-001, KDR-002]` | ✅ |
| `filter[gebieden]=Governance` | `[KDR-001, KDR-003]` | ✅ |
| `filter[gebieden][in]=Governance,Strategie` | `[KDR-001, KDR-003]` | ✅ |
| `filter[gebieden][in]=Privacy` | `[]` | ✅ (nothing has it) |
| `filter[gebieden][ne]=Informatiebeveiliging` | `[KDR-003]` | ✅ (was: all 3) |
| `filter[gebieden][contains]=Techno` | `[KDR-001, KDR-002]` | ✅ |
| `filter[status]=concept` (scalar) | `[KDR-003]` | ✅ unchanged |

*UI, real browser:*

1. The GEBIEDEN control renders as a single-line dropdown, not the tall native
listbox. Opening it shows a search box and all five options.
2. Selecting `Informatiebeveiliging` renders a removable **chip** with an ✕, and
the table filters 3 rows → the 2 that actually carry that value. This is the
exact interaction that produced "No organisatiebeoordelingskaders found." in the
report.
3. Adding `Strategie` gives URL
`?filter[gebieden][in]=Informatiebeveiliging,Strategie` — the documented `in`
form, not the broken `eq` — and returns all 3 rows.
4. Cold-loading `?filter[gebieden][in]=Strategie` hydrates the chip from the URL
and returns the 1 matching row, so deep links work.

*Regression tests proven to fail without the fix.* Reverting only the two source
files (tests kept) fails **7 of 8** list cases, including the two subtle ones:
the old code matched the stringified `[urgent blocker]` form, and matched
`contains` across an element boundary. Both are pinned as negative assertions.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: `matchesAnyCSV` removes a genuine duplicate (the `in`/`ne` membership loop)
where sharing sharpens the contract — complement semantics depend on both sides
reading the set identically. The element rule itself is not re-invented; it
matches `filter.matchList` and the in-package `propertyContains` that already
backs static `filters:`.

Security: no ACL surface touched. Filtering runs after the visibility-gated
read, so this changes which *already-authorized* rows survive a predicate — it
cannot widen what a principal may see. No new external input reaches a query.

Scratch reproduction harness was deleted after use; verification project lived
in `.ignored/` and was removed.

**Gates:** `go test ./...` ✅ · `just lint` (0 issues) ✅ · `just arch-lint` ✅ ·
`just comment-lint` ✅ · `just coverage-check` (78.3%, thresholds pass) ✅ · `just
plimsoll` ✅ · frontend `vitest` 2011/2011 ✅ · `vue-tsc` ✅ · `eslint` 0 errors ✅
· `prettier --check` ✅
