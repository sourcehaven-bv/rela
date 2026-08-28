---
id: PLAN-TFDIO6
type: planning-checklist
title: 'Planning: Render list cells and kanban card fields through the widget registry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: list/table cells (`EntityList.vue`, desktop `<td>` + mobile card) and kanban
card fields (`KanbanView.vue`, simple + swimlane boards) render read-only values
through the widget registry in `mode: 'display'`, with `text` as the universal
fallback. A schema-driven hint builder for dense surfaces.

OUT: inline-edit (TKT-IHCY7/HOIX1/GUPMK cover that); a `widget:` config key on
`ListColumn`/`KanbanCardField` (needs a backend validator counterpart —
follow-up); relation widgets; any new property type.

**Acceptance Criteria:**

1. Every built-in property type renders in a cell and a card, identical to the
pre-migration string output — except enums, which keep badging.
2. No console warnings on any built-in type at any row count.
3. Widget resolution happens once per column, not per cell.
4. Relation cells, ACL-locked cells, and the emptiness predicates are unchanged.

## Research

- [x] ~~Run `/research`~~ (N/A: no unknowns — the target pattern already exists in-tree)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the approach was determined by existing code, not by a
survey.

**Existing Solutions:**

No library involved; this is internal wiring. The decisive prior art is in-tree:

- `components/common/PropertyDisplay.vue:52-107` — the exact pattern to copy
(precomputed `rows` computed + `<component :is>` with an explicit
`:mode="'display'"`). `EntityDetail.vue` `fieldRowsFor` is a second instance.
- `widgets/registry.ts:92` `resolveFromHint` and `widgets/types.ts:69-93`
`WidgetRoutingHint` already exist for exactly this case, with a documented
rationale (RR-UD2B: view-side callers must not synthesise a fake `PropertyDef`).
- So this ticket is an unfinished migration, not new machinery. FEAT-72NR1 claims
five increments but only three tickets existed, all inline-edit; these two
surfaces were the gap.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add `densePropertyRoutingHint(propertyDef, name) -> DenseRoutingHint` to
`widgets/viewRouting.ts`. Each surface builds a `computed` Map of property →
`{component, hint}` (once per column/field), and the template renders
`<component :is>` with the ACL check as a sibling `v-if`.

The hint carries `preformatted`, so routing owns both *which widget* and *what
shape it wants*. Passthrough widgets (text, integer) get the formatted string;
widgets with their own display formatter (date, datetime, rrule, enum) get the
raw value.

**Alternatives rejected:**

- *Reuse `viewFieldRoutingHint`* — maps any truthy `propType` to `enum-list`,
which is bug-compatibility with pre-refactor Badge behaviour, not a type
mapping. Would badge every typed column in a table.
- *Use `registry.resolve(propertyDef)`* — routes `file` to FileWidget (an
`<img>` request per cell) and lets list-valued enums reach SelectWidget, which
`console.warn`s per row.
- *Extract a shared cell component* — would prevent future drift across the four
render sites, but widens the diff; noted in the ticket as optional.

**Files to modify:** `widgets/viewRouting.ts`,
`components/lists/EntityList.vue`, `views/KanbanView.vue` (+ the three test
files).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Entity property values* (server, ultimately user-authored): rendered as text
by every widget on this path — interpolated, never `v-html`. No new sink.
- *`data-entry.yaml` column/field config* (operator-authored): selects a
property name only; it cannot name a widget on this path (no `widget:` key on
`ListColumn`/`KanbanCardField`), so config cannot reach an arbitrary component.
Resolution is an allowlist: a closed `WidgetHintKind` union → a fixed registry
map.
- *Unknown property* (no schema entry): falls back to `text`, never throws.

**Security-Sensitive Operations:**

Read-side ACL is the one that matters. `isCellInaccessible` stays a **sibling**
`v-if` outside the widget, mirroring `PropertyDisplay`'s `InaccessibleField`, so
a locked property's value never enters `resolveCell` and never reaches a widget.
Verified in review that there is no third path. Pinned by a test.

No file access, auth, or crypto in this change.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1 → per-type mount tests at both EntityList sites and both kanban sites.
2. AC2 → a test rendering all built-in types asserting `console.warn` is unused.
3. AC3 → 50 rows × 3 columns asserts exactly 3 `resolveFromHint` calls.
4. AC4 → relation cell shows joined titles; inaccessible cell shows the lock;
empty column stays hidden on mobile.

Unit tests for the hint builder; component-mount integration tests for
rendering.

**Edge Cases:**

- Empty/null/missing: `null`, `undefined`, `''`, `[]` — must render blank, not a
placeholder (cells and detail views have opposite contracts here).
- `false` and `0` — set values, must render `No`/`0` and must NOT be dropped by
the emptiness predicates.
- List-valued variants of every scalar type — must not lose the type's formatter.
- Unparseable values (a `date` holding `'garbage'`).
- A property with no schema entry (`id` pseudo-column).
- Schema/data mismatch: scalar-declared enum holding an array.

**Negative Tests:**

- `file` must never resolve to FileWidget (no image requests from a table).
- `boolean` must never resolve to a checkbox in a cell.
- No hint may resolve to `text-list` on a dense surface.
Each fails loudly if someone "simplifies" the routing to `resolve(propertyDef)`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Four render sites, no shared component* → they can drift. Mitigated by
covering all four with tests (the mobile site had none before).
- *Widget contracts differ between detail and dense surfaces* — this was the
real risk and it materialised: MultiSelectWidget's em-dash and list-first
routing both leaked a detail-view contract into cells. Caught in review, fixed,
and pinned.
- *Dense mounting cost* — investigated up front: no widget has a lifecycle hook,
listener, observer, dynamic import, or fetch. Only FileWidget's `<img>` previews
are a genuine hazard, handled by routing `file` to text.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist created~~ (N/A: no user-facing behaviour to document)

**Documentation Impact:**

- [x] N/A — internal refactor. Rendering is unchanged for every existing config;
the one behaviour change (kanban cards gaining formatting, and `false`/`0` no
longer vanishing) is a bug fix, not a documented feature. No metamodel, CLI, or
config surface changes.

## Design Review

- [x] ~~Run `/design-review` before implementation~~ (N/A: design settled in
conversation with the requester, who chose the sequencing, the boolean
rendering, and the caching approach before implementation began)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A — `/code-review` ran after implementation and
produced RR-L5I3L1, RR-IPTBC7, RR-GHU9TE, RR-UTZM9Q, RR-2UAM9F, RR-01JKVJ,
RR-XEC2RD. Two criticals and two significants fixed; one significant deferred
with reason.
