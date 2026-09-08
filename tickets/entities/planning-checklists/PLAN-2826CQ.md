---
id: PLAN-2826CQ
type: planning-checklist
title: 'Planning: Make list rows and kanban cards behave as real links'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: the two surfaces the issue names — entity list rows and kanban cards — get a
real `<a href>` so the browser's own navigation affordances work. That is FOUR
render sites, not two: each surface has a second template that duplicates its
markup.

- `EntityList.vue`: the desktop `<table>` row **and** the mobile card layout.
- `KanbanView.vue`: the simple board **and** the swimlane board.

Counting these as two surfaces rather than four is the trap here — a fix applied
to one template of a pair leaves the other silently click-only.

OUT:
- other click-to-navigate surfaces (command palette already uses
`entityDetailHref`; graph/timeline views are not mentioned in the issue and have
different hit-target semantics).
- changing where a click *goes*. Routing is unchanged; only the markup that
expresses it changes.
- keyboard focus/tab-order work. The anchor makes rows focusable, which is a
side benefit, but no keyboard behaviour is being designed here.

**Acceptance Criteria:**

1. Each list row renders a real `<a href>` pointing at the entity.
*Test:* mount `EntityList` with two entities, assert two `a.row-link` with the
expected hrefs.
2. Exactly one anchor per row, not one per cell.
*Test:* mount with a multi-column list, assert anchor count equals row count.
3. The href preserves list state (active filter, search query) so an opened tab
lands in the same context. *Test:* mount with filter + `q` set, assert both
appear in the href query.
4. Plain left-click still navigates in place (no full page load).
*Test:* trigger `click`, assert `router.push` called with the same location.
5. Each kanban card renders a real `<a href>` for its id/title.
*Test:* mount a board with two cards, assert two `a.card-link` with hrefs.
6. Boards configured with an `edit_form` point at the form route — href and
click agree. *Test:* mount with `edit_form: 'ticket-edit'`, assert the form
href.
7. Kanban drag-and-drop keeps working.
*Test:* assert the card is `draggable="true"` and the anchor
`draggable="false"`.
8. The mobile card layout links its title too.
*Test:* stub `matchMedia` to mobile, assert `.mobile-card-list` is the rendered
template and each card title is an `<a href>`.
9. The swimlane board links its cards too.
*Test:* mount with `swimlane_property`, assert the card anchor and href.

AC8 and AC9 exist because each is a separate template. Their tests must fail
when only *their* site regresses — verified by mutation, see IMPL-6CAAN2.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, well-bounded UI change.

**Existing Solutions:**

- No library needed; this is a markup change against vue-router.
- Prior art in this codebase: `entityDetailHref` in `utils/entityRoute.ts`,
already used by the command palette. Its call site in `EntityList` even carries
the comment *"Centralised so right-click / middle-click open through a real `<a
href>` on the row markup elsewhere."* — the helper landed for the palette and
the row markup never followed. This ticket finishes that.
- `RouterLink` was considered and rejected (see Approach).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Extract the "where does this row point" decision into one function per
component, then use it from *both* the click handler and the `href`, so the two
cannot drift:

- `EntityList.vue`: `entityRowLocation(entity) → {path, query} | null`;
`navigateToEntity` pushes it, `entityRowHref` serialises it.
- `KanbanView.vue`: `cardHref(entity)`; `openCard` pushes what it returns.

Anchor placement:

- list: wrap the *first column's cell content*. Wrapping the whole `<tr>` is not
valid HTML; wrapping every cell would emit N anchors per row and make
right-click behaviour depend on which column you hit.
- kanban: wrap the id + title only, not the card. The card is the drag handle;
an anchor around a draggable element makes the browser start a link drag. Belt
and braces: `draggable="false"` on the anchor.

`@click.stop` on the anchor so the existing row/card handler still owns the
plain left click and the SPA navigates in place; the browser keeps modified
clicks for itself.

Invisible-link CSS: `.row-link { display: contents; color: inherit;
text-decoration: none; }` — `display: contents` keeps the anchor out of table
layout entirely. `.card-link { display: block; ... }`.

**Alternatives considered:**

- `<RouterLink>` — rejected: it renders its own `<a>` with its own click
handling, which would double up with the existing row handler, and its `custom`
mode gives back exactly the `href` we are computing anyway.
- `router.resolve()` to build the href — *tried and rejected*: it broke 34
existing tests whose router mocks expose only `push`/`replace`.
`URLSearchParams` is pure, needs no router instance, and is faster.

**Files to modify:**

- `frontend/src/components/lists/EntityList.vue` (table row + mobile card)
- `frontend/src/views/KanbanView.vue` (simple board + swimlane board)
- `frontend/src/components/lists/EntityList.rowlink.test.ts` (new)
- `frontend/src/views/KanbanView.test.ts`

Not changed, and why: `EntityDetail.vue`, `RelationCards.vue`, `SidePanel.vue`
and `EntityPreviewModal.vue` also navigate on click. They are out of scope — the
issue names lists and kanban cards, and those surfaces are inline
cross-references inside a form or modal with different hit-target semantics.
Worth a follow-up ticket, not scope creep on this one.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- entity id and type, and the list's filter/`q` state — all already rendered by
these components. They now additionally reach an `href` attribute.
- The hrefs are same-origin app paths built from a fixed prefix plus
`URLSearchParams`, which percent-encodes. There is no user-supplied *scheme*, so
`javascript:` URLs are not constructible here; Vue attribute binding escapes the
value into the attribute besides.
- No new auth/file/crypto surface. Visibility is unchanged: an href is only
rendered for an entity the list already returned, and following it hits the same
route with the same server-side checks.

**Security-Sensitive Operations:** none introduced. The change is markup over
data already on screen.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one per acceptance criterion, listed above.

**Edge Cases:**

- entity with no navigable target → `entityRowLocation` returns `null`, cell
renders without an anchor (no dead `href=""`).
- list with a single column → the first-column rule still yields exactly one
anchor.
- board with `edit_form` → href points at the form route, matching the click.
- card mid-drag → anchor is `draggable="false"` so the card's HTML5 drag wins.

**Negative Tests:**

- the anchor-count test fails if anchors are emitted per cell.
- the drag test fails if the anchor becomes draggable.
- all new tests were mutation-checked: the fix was broken and each test
confirmed to redden. See IMPL-6CAAN2 for the table.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Anchor breaks table layout.* Mitigated by `display: contents`, and by the
full 2060-test suite staying green.
- *Anchor breaks kanban drag-and-drop.* This is the real one: it is a pointer
interaction that unit tests can only approximate. Mitigated by scoping the
anchor to the text rather than the card, `draggable="false"`, and a test that
pins that boundary so a later refactor that widens the anchor goes red.
- *href and click drift apart* — the failure mode where middle-click lands
somewhere different from left-click. Mitigated structurally: both read from one
function.
- *A duplicate template is missed.* This risk MATERIALISED: the first pass
covered the desktop table and the simple board and left the mobile layout and
the swimlane board click-only. Caught by grepping the remaining
click-to-navigate handlers, not by any test — so each of the four sites now
carries a test that reddens when only that site regresses.

**Effort:** m

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — no documented behaviour changes. Where a click goes is unchanged;
the browser affordances that now work (open-in-new-tab, Cmd-click) are standard
link behaviour users already expect, and `docs/data-entry.md` never documented
their absence.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** no critical or significant findings. The two points
raised during review — anchor placement in the table (one per row, not per cell)
and the drag-and-drop interaction on cards — are folded into the Approach and
each pinned by a test.
