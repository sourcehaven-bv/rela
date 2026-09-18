---
id: BUG-MYN56J
type: bug
title: Kanban static filters silently ignore six of the nine operators the validator accepts
description: 'A kanban''s filters: are applied client-side by a 3-case switch whose default arm returns true. The Go validator accepts 9 operators; the board implements 3. The other six (~, in, <, <=, >, >=) are silent no-ops. Same failure class as BUG-F1LTV0, inverted: silently-everything instead of silently-nothing.'
priority: high
status: backlog
---

## Summary

A kanban's `filters:` are applied **client-side only**, by a three-case switch
whose `default` arm returns `true`. The Go config validator accepts nine
operators. The board implements three. The other six — `~`, `in`, `<`, `<=`,
`>`, `>=` — are **silent no-ops**: the filter appears in config, passes
validation, and does nothing.

This is `BUG-F1LTV0`'s failure class still live, inverted. There the board
silently showed *nothing*; here it silently shows *everything*.

## Evidence

`frontend/src/views/KanbanView.vue:222-252`:

```js
const filteredEntities = computed(() => {
  let result = [...entities.value]
  if (kanbanConfig.value?.filters) {
    for (const filter of kanbanConfig.value.filters) {
      result = result.filter((entity) => {
        const val = String(entity.properties[filter.property] || '')
        switch (filter.operator) {
          case '=':
          case '==':
            return val === filter.value
          case '!=':
            return val !== filter.value
          default:
            return true      // ← six accepted operators land here
        }
      })
    }
  }
  ...
```

The validator that accepts all nine —
`internal/dataentryconfig/validate.go:62-77` (`validFilterOperators`), enforced
for kanbans at `validate.go:1732-1746`:

```go
for i, f := range kanban.Filters {
	if !validFilterOperators[f.Operator] {
		errs = append(errs, fmt.Sprintf(
			"kanban %q: filters[%d] has invalid operator %q (valid: %s)", ...))
	}
	...
}
```

So `operator: "<="` on a kanban passes startup validation and is then discarded
in the browser with no error, no console warning, and the filter chip still
rendering as applied.

## Two further defects in the same block

1. **List properties are flattened before comparison.**
`String(entity.properties[p] || '')` turns a multi-select into `"a,b"`, so even
`=` compares against a joined string. This is the same defect `BUG-AMK38R` fixed
on the list path (where `applyV1Filters` flattened with `fmt.Sprintf("%v")`
while `filter.matchList` handled it correctly).
2. **`$today` and friends are never substituted.** Variable substitution is
documented for static filters (`docs/data-entry.md`, "Variable substitution in
filter values") and implemented server-side. The kanban path never reaches a
server filter, so `value: $today` compares literally against the string
`"$today"` and matches nothing.

The user filter controls immediately below (`KanbanView.vue:244-249`) are
equality-only too, with the same flattening.

## Root cause (why the class recurs)

`BUG-F1LTV0`'s why4 already named it:

> Four independent hand-maintained operator sets (docs table, validator map,
> SPA OPERATOR_MAP, API switch) existed with no cross-layer contract test, so
> each layer's silent-fallback behavior masked the others' gaps.

The kanban is a **fifth** such set, and the one with no server layer behind it
at all. `AM-filter-operator-set-pin` pins the validator against the docs table
and the list path; nothing pins the kanban switch against the validator.

## Why the kanban has no server filtering

There is no Go kanban handler. The board fetches the **entire entity type** and
partitions it in JavaScript:

- `KanbanView.vue:122-128` — `boardParams` carries only `include` and `world`.
Never filters, never sort, never page.
- `KanbanView.vue:140` — `listAllEntities(config.entity, boardParams.value, signal)`,
which loops pages with a 50-page runaway guard
(`frontend/src/api/entities.ts:59`).
- The comment at `KanbanView.vue:118-121` records why it must fetch everything:
a single page would silently drop page 2+ (`BUG-5OAQUG`).
- `queryplan.listIndexSpec` (`internal/queryplan/queryplan.go:344`) walks
`cfg.Lists` only, so kanbans contribute no derived indexes either.

The sizing assumption on record is ~5,000 entities (`KanbanView.vue:153`),
surfaced as a visible truncation banner rather than swallowed
(`KanbanView.vue:155-158`).

## Reproduction

1. Add to any kanban in `data-entry.yaml`:
   ```yaml
   filters:
     - property: <a date property>
       operator: ">="
       value: "2030-01-01"
   ```
2. `rela validate` passes; the server starts.
3. Open the board. Every card is still shown, including ones whose date is far
below the bound.

Expected: either the filter is applied, or config load refuses the operator on a
kanban.

## Fix direction (to settle in analysis)

Two honest options, and the choice interacts with **TKT-LPLZ1V**:

- **(a) Narrow the validator for kanbans** to the three operators the board can
actually evaluate, making the rest a load error. Small, immediate, honest — and
strictly better than today's silence. Costs authors the six operators on boards.
- **(b) Give the kanban a server read path**, reusing `listPage` /
`scopedSortedEntities` so one Go matcher serves both lists and boards. Fixes the
operator gap, the list-flattening and the `$today` gap together, and removes the
load-the-whole-type behaviour. Larger, and it is close to what **TKT-LPLZ1V**
needs anyway.

Either way, add the missing cross-layer pin: a contract test asserting the
kanban's evaluable operator set equals the set its validator accepts, so a fifth
divergence cannot open silently.

## Notes

Found while scoping **TKT-LPLZ1V** (condition expressions on list/kanban views).
Filed separately because it is a shipping defect independent of that feature,
and because TKT-LPLZ1V's design should not quietly inherit a broken baseline.
