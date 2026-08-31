---
id: IMPL-6CAAN2
type: implementation-checklist
title: 'Implementation: Make list rows and kanban cards behave as real links'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`EntityList.vue`: extracted `entityRowLocation(entity)` — the single place that
decides where a row points, returning `{path, query} | null`. `navigateToEntity`
pushes it; `entityRowHref` serialises it to a URL string for the `href`. Both
click and href therefore go through one decision, so they cannot drift apart.

`KanbanView.vue`: same shape via `cardHref(entity)`; `openCard` pushes what
`cardHref` returns.

### Four render sites, not two

The first pass linked the desktop table row and the simple kanban card, and
missed two templates that render the same thing elsewhere:

- `EntityList.vue` **mobile card layout** (`v-if="isMobile"`) — a separate
template from the `<table>`, with its own title element.
- `KanbanView.vue` **swimlane board** — a separate template from the simple
board, with its own card markup.

Both were found by grepping for the remaining click-to-navigate handlers rather
than by any test, which is the point: the four sites are structural duplicates,
so each needs its own test or it can regress alone. Each now has one, and each
of those tests was mutation-checked against *its own* site (reverting the
swimlane anchor reddens only the swimlane test, and so on).

Edge cases handled:

- rows with no navigable target (`entityRowLocation` returns `null`) render a
plain cell, not a dead `<a>` — pinned by the anchor-count test.
- boards/lists configured with an `edit_form` point at the form route, so href
and click agree there too.
- the anchor carries `draggable="false"` on kanban cards. The card itself is the
HTML5 drag handle; without this the browser starts a *link* drag from the title
and the column drop never fires.

No error paths are introduced — both helpers are pure functions over data the
component already holds.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Kanban tests reuse the file's existing `mountBoard` / `makeTicket` helpers. The
list tests build entities through the same shape the other `EntityList` specs
use.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Every behavioural claim was mutation-checked — the fix was broken and the test
re-run to confirm it reddens, then restored:

| mutation | expected | observed |
| --- | --- | --- |
| `v-if="colIndex === 0 && entityRowHref(entity)"` → `v-if="false"` (drop the row anchor) | row-link tests fail | 4 failed (4) |
| drop `:href` from the kanban anchor | card href tests fail | 2 failed, 19 passed |
| drop `draggable="false"` from the kanban anchor | drag-boundary test fails | 1 failed, 20 passed |
| revert the SWIMLANE card anchor only | swimlane test fails, simple-board tests stay green | 1 failed, 21 passed |
| `v-if="entityRowHref(entity)"` → `v-if="false"` on the mobile card title | both mobile tests fail, desktop tests stay green | 2 failed, 4 passed |
| all restored | green | 22 kanban / 6 row-link, all passing |

The first `draggable` mutation attempt edited a *comment* rather than the
attribute and the suite stayed green — a green that proved nothing. Re-ran
against the real attribute to get the failure above.

Note the swimlane and mobile mutations are scoped to a single render site each
and redden only that site's test — that is what shows the four sites are
independently covered rather than one test standing in for all of them.

`npm run build` (which runs `vue-tsc -b`) clean; eslint reports no findings in
either changed file.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: the `{path, query}` decision was already duplicated between the click
handler and (implicitly) the href I was about to add, so it was extracted to
`entityRowLocation`. It is deliberately *not* shared across `EntityList` and
`KanbanView` — the two build different routes from different config, and a
shared helper would have to take both shapes to say nothing extra.

The table cell body was briefly duplicated between a linked and an unlinked
branch. Replaced with one dynamic wrapper (`:is="… ? 'a' : 'span'"`) so the cell
body is written once — two copies of that markup is precisely how the linked
first column and the other columns would drift apart.

Three link CSS classes, each earning its place: `.row-link` and `.row-cell` use
`display: contents` so the table wrapper adds no box; `.plain-link` does not,
because the mobile card title carries `flex: 1; min-width: 0` that `display:
contents` would discard.

`entityRowHref` builds its query with `URLSearchParams` rather than
`router.resolve()`. `resolve()` was tried first and broke 34 existing tests
whose router mocks expose only `push`/`replace`; `URLSearchParams` is also pure
and needs no router instance.
