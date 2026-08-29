---
id: BUG-AMK38R
type: bug
title: Multi-enum list filter renders a native listbox and never matches any row
description: 'The list-view filter control for a multi-valued enum property (list: true with enum values:) renders as a raw browser <select multiple> listbox, and selecting any value returns zero rows. Root cause: applyV1Filters flattens list property values with fmt.Sprintf("%v") before comparing, so a slice becomes "[Governance Technologie]" and can never equal a single selected value.'
priority: high
effort: m
why1: Selecting a value in a multi-enum filter returns no rows, and the control renders as an unstyled native listbox.
why2: applyV1Filters (internal/dataentry/api_v1.go:1839) flattens every property value with fmt.Sprintf("%v", propVal) before comparing, so a list property becomes the Go-formatted slice "[Governance Technologie]" and never equals a single selected value. Separately, FilterBar.vue:271 comma-joins selections and emits them under the default eq operator, which is unrepresentable on the wire.
why3: 'The dataentry list handler grew its own inline comparison loop instead of using the correct implementation that already exists THREE times over: filter.matchList, propmatch.equalsTarget, and — in its own package — dataentry.propertyContains, which backs the static config-authored filters: path. So static filters match list properties correctly while the identical predicate arriving as a filter[...] query param does not. The FilterBar likewise predates the widget registry and never adopted TagSelect, which every edit form reaches through MultiSelectWidget.'
why4: 'List-typed properties were treated as an afterthought on the read path: the filter code was written against scalar values and a slice silently degrades to a printable string rather than failing, so nothing surfaced the gap. The identical defect was already found and fixed once on the WRITE path (validator ''only knew how to inspect scalar string values'', TestV1Affordance_PatchFilteredListEnum_Forbidden) without anyone auditing the read path for the same class.'
why5: There is no single enforced comparison seam for property values. Four parallel implementations of 'does this property value match this string' coexist (filter.matchList, propmatch.equalsTarget, dataentry.propertyContains, applyV1Filters); three are correct and the fourth is not, and nothing structurally prevents a call site from hand-rolling a fifth. This is the same class already flagged by TODO(TKT-HFEKVN) about the temporal-layout duplication in this very function, whose comment notes the copies 'already disagree' and that the failure mode is invisible because it changes which rows come back rather than erroring.
prevention: 'Done in this fix: the dataentry list filter compares per element through one shared helper, propertyContains is now a one-liner over that same helper (so the static filters: path and the dynamic filter[...] path are literally the same code rather than two implementations asserted to agree), and HTTP-level table tests pin eq/ne/in/contains against list-typed properties plus the nil, comma-bearing-value and ordered-operator edge cases. internal/filter.matchList — the reference implementation, previously untested — now has direct unit tests running every case against both []string and []any. Still open: TKT-UTJ24Z converges applyV1Filters onto internal/filter so a fifth copy of the matching rule cannot be added, and carries the two pre-existing defects this review surfaced (empty in/ne is not the complement of a populated one; in/ne trim the filter side but not the property side). A lint or arch guard flagging fmt.Sprintf("%v") used for property-value COMPARISON remains worth considering — every defect found here, including the two the review caught in the fix itself, was a variation on rendering-used-as-comparison.'
status: done
---

## Symptom

On an `organisatiebeoordelingskader` list with a `gebieden` (list-of-enum)
filter control:

- The **GEBIEDEN** filter renders as a raw browser `<select multiple>` — a small scrolling listbox with OS-blue highlight, no chips, no search, no clear affordance, clipped options.
- Rows visibly carry `Informatiebeveiliging` in the GEBIEDEN column, but selecting it in the filter yields **"No organisatiebeoordelingskaders found."**

## Bug 1 — filtering never matches (functional, severe)

`applyV1Filters` flattens every property value before comparing:

```go
// internal/dataentry/api_v1.go:1839
propStr := fmt.Sprintf("%v", propVal)
```

For a list property this renders the Go-formatted slice. Verified empirically:

```
fmt.Sprintf("%v", []any{"Governance","Technologie"}) == "[Governance Technologie]"
eq "Technologie" -> false
```

No selection can ever match — the filter is **structurally incapable** of
returning a row, not merely mis-scoped. `contains` accidentally half-works
(substring of the flattened form, including cross-element false positives);
`eq`/`ne`/`in` cannot match at all.

The correct semantics already exist in-tree and are bypassed:

- `propmatch.equalsTarget` (`internal/propmatch/propmatch.go:142`) — handles `[]string`/`[]any` as "matches when ANY element does"
- `filter.matchList` (`internal/filter/match.go:95`) — same rule

This is the **read-path twin of a defect already fixed on the write path**,
where the validator "only knew how to inspect scalar string values" (see
`TestV1Affordance_PatchFilteredListEnum_Forbidden`,
`internal/dataentry/api_v1_test.go:4786`).

Note `docs/data-entry.md:1073` documents `filter[tags][in][]=urgent` — a
list-typed property — as the canonical multi-value example. The documented use
case has never worked.

## Bug 2 — native listbox widget (UX)

```vue
<!-- frontend/src/components/lists/FilterBar.vue:295 -->
<select v-else-if="filter.widget === 'multi-select'" multiple ...>
```

Styled only by `min-height: 80px`. Meanwhile `TagSelect.vue` (SlimSelect —
chips, search, deselect, option labels, option verdicts) already exists and is
the standard multi-select **everywhere else**: edit forms reach it via
`MultiSelectWidget` → `registry.ts`, using the *same*
`schemaStore.resolveOptionLabels` FilterBar already calls at
`FilterBar.vue:101`.

This is an inconsistency to remove, not a component to build.

## Bug 3 — multi-value selection is unrepresentable on the wire

```js
// FilterBar.vue:271
localFilters.value[key] = selected.join(',')
```

Emitted under the default `=`/`eq` form. Even with Bug 1 fixed, `eq` against
`"a,b"` matches nothing. The backend **does** support multi-value via the `in`
operator (`api_v1.go:1820`, which collects repeated `filter[tags][in][]=`
params), but FilterBar never emits `in` — so the widget cannot express what it
lets the user select.

## Why all three

A fix addressing only Bug 1 leaves multi-selection broken; only Bug 2 yields a
prettier control that still returns nothing.

---

# Long-term fix: the design decision

The instinct is "delete the duplicate matcher, delegate to `filter.Match`." That
is the right *direction* — but investigation shows full delegation is not a
drop-in, and doing it naively would trade a visible bug for three silent
behavior changes. The structural problem and the user-facing bug should be fixed
together but **not conflated**.

### Why `applyV1Filters` cannot simply call `filter.Match` today

Three concrete divergences, each of which would silently alter behavior:

1. **Operator set mismatch.** `filter.Operator` has exactly eight values
(`=`, `!=`, `<`, `<=`, `>`, `>=`, `=~`, `~`). It has **no `in` and no
`contains`** — verified: `OpIn`/`OpContains` appear nowhere in `internal/`. The
HTTP API supports both and `docs/data-entry.md:1081` documents them as part of
the public operator set. Delegating means either adding two operators to the
shared core (used by CalDAV, feeds, views, CLI) or dropping documented API
behavior.
2. **`eq` is glob-aware in `filter`, literal in HTTP.** `matchString`
(`match.go:207-215`) honors `filter.IsGlob`, so a value containing `*` or `?`
becomes a pattern. The HTTP `eq` is literal string equality. A filter value with
a `*` would silently change meaning.
3. **`filter.Match` rejects ordered operators on string properties**
(`validateOperatorForType`, `match.go:157-173`), whereas the HTTP path allows
`gte` on anything and falls back to lexicographic `compareValues`. Requests that
work today would start returning 400.

### The precedent this belongs to

This is the *second* documented instance of the same structural failure in the
very same function. `internal/dataentry/helpers.go:72` carries
**`TODO(TKT-HFEKVN)`**: the temporal-layout list is the THIRD copy, the copies
"already disagree," and the failure mode is explicitly noted as invisible — "it
does not error, it changes which rows come back."

`applyV1Filters` is therefore a known duplication site with a known cost. That
argues for consolidation as the destination, and equally for treating it as
deliberate work with its own tests rather than a side effect of a UI bugfix.

### Chosen approach — fix correctly now, converge deliberately

**Step 1 (this bug): make list values a first-class case, once.** Introduce a
single value-comparison seam in `internal/dataentry` that every operator branch
in `applyV1Filters` goes through, replacing the `propStr := fmt.Sprintf("%v",
propVal)` flattening at `api_v1.go:1837`. Semantics match `filter.matchList`
exactly — ANY-element for `eq`/`in`/ `contains`, NO-element for `ne` — reusing
`propmatch` for the element comparison so the element-level rule is not
re-invented. Scalar behavior is byte-for-byte unchanged; only the list case,
which currently cannot match at all, changes. There is no back-compat risk
because there is no working behavior to preserve.

**Step 2 (this bug): make the widget honest.** Render `TagSelect` for `widget
=== 'multi-select'` and emit `op: 'in'` so the wire form becomes
`filter[p][in]=a,b` — the operator the backend already defines for multi-value
and the one the docs already advertise.

**Step 3 (follow-up ticket, not this bug): converge on one matcher.** Reconcile
the operator sets and the three divergences above, then delete the duplicate.
Sequenced after TKT-HFEKVN, which already owns the temporal half of this same
function — doing them in the wrong order means touching `compareValues` twice.

**Why not fold Step 3 in:** it changes behavior for CalDAV, feeds, views and the
CLI — every `filter.Match` consumer — none of which are broken today. That is a
distinct blast radius from a list filter that returns nothing, and bundling them
means a regression there is indistinguishable from this fix.

### Guard against recurrence

A local seam is only an improvement if it cannot be bypassed the way `propmatch`
was. The measure is a test that pins list semantics at the HTTP boundary for
**every** operator, so a future branch added without list handling fails rather
than silently returning the wrong rows.

## Scope note

This list-filter path is **in-memory only** — no SQL/pgstore pushdown is
involved (`api_v1.go:436` notes relation-title pushdown as future work), so the
fix is contained to the Go handler plus the Vue component.

## Test gap

No test covers filtering a list-typed property end-to-end through the HTTP list
handler — every existing case in `api_v1_test.go` seeds a scalar string
property. `internal/filter.matchList`, the correct implementation, has **no
direct unit test either**. That is how this shipped.
