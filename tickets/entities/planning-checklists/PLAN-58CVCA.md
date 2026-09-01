---
id: PLAN-58CVCA
type: planning-checklist
title: 'Planning: Ctrl/Cmd-click (and middle-click) should open data-entry rows and cards in a new browser tab'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The data-entry SPA navigates from non-anchor elements (`<tr @click>`, `<div
@click>`, `<li @click>`, `<span @click>`) by calling `router.push`. With no `<a
href>` the browser has nothing to act on: no background tab on cmd/ctrl-click,
nothing on middle-click, no "Open link in new tab" context menu, no hover URL
preview.

**The fix is to use real links.** Not a modifier-key interception layer — an `<a
href>` / `<RouterLink>`, which gets every one of those behaviours from the
browser for free. `RouterLink` already implements the whole modifier-click
contract internally (`guardEvent`), so for most surfaces there is no guard code
to write at all.

**Revised after review** (an earlier draft proposed a `wantsNewTab()` helper at
all 10 surfaces — that was over-engineered; 9 of 10 need only an anchor).

**Scope:**

IN — replace programmatic `router.push` with a real link:

| File | Element | Conversion |
|---|---|---|
| `views/KanbanView.vue` (:593, :661) | `.kanban-card` | whole card → `RouterLink` (no interactive descendants; the only `<button>` is the toolbar "New" at :525, outside) |
| `views/SearchView.vue` (:435) | `.result-item` | whole item → `RouterLink` |
| `components/forms/SidePanel.vue` (:147, :161) | `.entity-list-item`, `.entity-card` | whole entry → `RouterLink` (no per-item controls) |
| `components/common/IssuesTable.vue` (:105, :175) | `.entity-title` span, **desktop AND mobile** | → `RouterLink`; drop `role="button"`/`tabindex`/both keydown handlers |
| `components/entity/EntityDetail.vue` (:1092) | `<a class="list-link">` — already an anchor, **missing `href`** | add `:to`; smallest fix in the ticket |
| `components/entity/EntityDetail.vue` (:1001, :1028) | `.card-header` | anchor the **title spans only** — the header contains `.edit-btn` (`@click.stop`, :1036) and an `<a>` may not contain a `<button>` |
| `components/forms/RelationCards.vue` (:562, :565) | `.entity-id` / `.entity-title` spans | → `RouterLink` + `draggable="false"` (inside a drag source) |
| `components/ui/CommandPaletteModal.vue` (:291) | `.cmdk-option` | anchor inside the `<li>`, `tabindex="-1"` (it is a `role="option"` in a `role="listbox"`) |
| `components/lists/EntityList.vue` (:884) | `.mobile-card` | anchor the **title span only** — card contains a delete `<button>` (:892) |
| `components/lists/EntityList.vue` (:993) | `<tr class="entity-row">` | title-cell anchor + **stretched-link overlay** (see below) |

OUT:

- **Calendar event chips** — the click opens `EntityPreviewModal`, not a route.
No URL exists to put in a tab. Unchanged.
- Surfaces already using `<router-link>`: `DashboardView.vue`,
`NextActionCard/Offers.vue`, `Sidebar.vue`, `StatusBar.vue`, `BackButton.vue`,
`+ New` buttons. Already correct.
- `useDocumentClicks.ts` — already correct.
- Non-navigating clicks: checkboxes, delete buttons, sort headers, filter chips,
collapse toggles, drag handles, pagination.
- Keyboard shortcuts (`j`/`k`/Enter) — unchanged.
- **Converting tables to CSS grid — explicitly rejected.** See below.

**Decision: no table → CSS-grid conversion.**

Counted: 7 `<table>` elements across 5 files, but **only ONE has clickable
rows** (`EntityList`). ConflictsView (×2), DashboardView, EntityDetail (×2) and
IssuesTable have zero `<tr @click>`. So there is no "table problem" category —
there is one table. Grid was considered because a grid row can be a single `<a
display:grid>`, needing no overlay. Rejected because:

- It discards native table semantics (screen readers announce "row 3 of 47,
column Status" from the markup); a grid must hand-rebuild
`role="table"`/`"row"`/`"cell"`, which is easier to get subtly wrong.
- It discards content-based column sizing. `EntityList` columns are configured
per-list in `data-entry.yaml`, so the count is unknown at build time — a grid
would need a runtime-generated `grid-template-columns` and lose auto-fit.
- It is a large markup+CSS refactor to solve what 4 lines of CSS solve.

**Precedent already in-repo:** `DashboardView.vue:263-272` keeps its `<table>`
and puts a `<router-link>` in the `<td>`. That is this codebase's existing
answer for navigating table cells, and it ctrl-clicks correctly today. We follow
it; the only difference in `EntityList` is that the whole *row* is the target,
not one cell — hence the overlay.

**Decision: stretched-link for the one clickable table (approved).**

HTML forbids an `<a>` wrapping or nesting directly inside a `<tr>` — the parser
discards a misplaced anchor. So a row-wide link is not expressible directly.
Chosen: a single title-cell anchor stretched over the row.

```css
.entity-row { position: relative; }
.entity-row .row-link::after { content: ''; position: absolute; inset: 0; }
.entity-row .select-cell, .entity-row .actions-cell { position: relative; z-index: 1; }
```

One real link per row in the a11y tree, whole row cmd-clickable, native table
semantics kept. **Accepted cost: text selection within a row is blocked** —
approved, since selecting text out of a list view is not a common action here.

**Acceptance Criteria:**

1. Cmd/Ctrl-click on a list row (anywhere), kanban card, search result,
side-panel entry, relation-card title, issues-table entity cell (desktop and
mobile), palette option and entity-detail card/list entry opens the target in a
new background tab; the current tab does NOT navigate.
2. Middle-click does the same on those surfaces.
3. Shift-click opens a new window.
4. Right-click offers "Open link in new tab" — a real `a[href]` is in the DOM.
5. Plain left-click is unchanged: in-SPA navigation, no full reload, identical
destination **including query params**.
6. Nested controls unaffected: row checkbox, delete button, kanban drag,
relation-card drag-reorder, side-panel add, palette roving focus.
7. The href carries the SAME query the programmatic push built (list
`from`/`scope`/`sort`/`filter[*]`/`q`; search `from=search`+`q`; kanban
`edit_form` branch).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the fix is "use the platform's link element"; a research
entity would add nothing.

**Existing Solutions:**

- **`DashboardView.vue:263-272`** — in-repo precedent: `<router-link>` inside a
`<td>`, table kept. Directly reused for `IssuesTable`.
- **vue-router `<RouterLink>`** — already implements the modifier-click contract
(`guardEvent`: metaKey/altKey/ctrlKey/shiftKey, `button !== 0`,
`defaultPrevented`). This is why no custom helper is needed at 9 of 10 sites.
Where custom click logic must coexist with a link, `<RouterLink custom v-slot="{
href, navigate }">` supplies both without hand-rolling the guard.
- **`useDocumentClicks.ts:26-40`** — correct modifier handling for rendered-doc
links; the reference for the ONE remaining hand-written guard (the `<tr>`).
- **`utils/entityRoute.ts`** — `entityDetailHref()` with `cellLink` priority,
returns `''` for empty type/id. Imported by `EntityList.vue:14`,
`CommandPaletteModal.vue:24` **and `EntityDetail.vue:16`** (a review claim that
EntityDetail does not import it was checked and is wrong).
- **Bootstrap `.stretched-link`** — the overlay pattern; `display: contents` is
already used 4× in this repo, so neither idiom is foreign here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*1. One shared target function per surface (RR-6PBTF1 — mandatory, not
optional).*

Each surface exposes `entityTarget(entity): RouteLocationRaw` returning path AND
query. `router.push(target)` and the link's `:to="target"` consume the SAME
value. This is the highest-risk item: `EntityList.navigateToEntity` (`:485-541`)
builds ~40 lines of reactive query state (`from`, `scope`, `sort` from live
`sortSpecs` else `default_sort`, every `filter[*]` incl. arrays, `q`). A
separately-built href would silently drop all of it — both paths still render a
page, so the divergence is invisible in review while `useScopeNavigation`
prev/next dies and the back target is wrong.

Binding `:to` (a location object) rather than a pre-stringified href also means
vue-router serializes the query and applies `BASE_URL` (`router/index.ts:116`
uses `createWebHistory(import.meta.env.BASE_URL)`).

*2. Real links, element chosen by what HTML allows.*

An `<a>` may not contain interactive content, and may not wrap a `<tr>`. Per
surface: wrap the whole container where it has no interactive descendants
(kanban, search result, side-panel entry); otherwise wrap the title/identity
element only (EntityList mobile card, EntityDetail card-header, RelationCards);
for the `<tr>`, title-cell anchor + overlay.

*3. `draggable="false"` on anchors inside drag sources (RR-NPDW9A).*

Anchors are natively draggable. `RelationCards.onDragStart` (`:458`)
early-returns when `!isOrderable`, so without this an anchor there gets the
browser's default link-drag — a behaviour that does not exist today. Kanban
cards need it too: `:draggable="false"` on the card does not stop an anchor
child being independently draggable.

*4. The one hand-written guard.* Only the `<tr>` keeps a click handler alongside
a link (the overlay anchor covers the row, but the handler remains for the
plain-click path). It early-returns on modifier/non-primary clicks. Named
`shouldDeferToBrowser` — NOT `wantsNewTab` (RR-Z0AHAY: it also covers
`defaultPrevented`, which means "nothing should happen", the opposite of "open a
tab").

**Files to modify:**

- `frontend/src/utils/openIntent.ts` (+ test) — NEW, small: `shouldDeferToBrowser`.
- `frontend/src/components/lists/EntityList.vue` — `entityTarget()`; title-cell
anchor + overlay CSS; mobile-card title anchor; fix the stale comment at :532.
- `frontend/src/views/KanbanView.vue` — cards → `RouterLink` + `draggable="false"`.
- `frontend/src/views/SearchView.vue` — result → `RouterLink` with scope query.
- `frontend/src/components/forms/SidePanel.vue` — entries → `RouterLink`.
- `frontend/src/components/forms/RelationCards.vue` — id/title spans →
`RouterLink` + `draggable="false"`.
- `frontend/src/components/common/IssuesTable.vue` — desktop **and mobile**
spans → `RouterLink`; remove role/tabindex/keydown.
- `frontend/src/components/entity/EntityDetail.vue` — `:to` on `.list-link`;
title-span anchors in both card-headers (keep `.edit-btn` a sibling; do not move
the header handler — TKT-IHC7C / RR-FC1B).
- `frontend/src/components/ui/CommandPaletteModal.vue` — anchor with
`tabindex="-1"`; keep palette open on modifier-click.
- `e2e/pages/*.page.ts` + `e2e/tests/open-new-tab.spec.ts` — NEW.

**Alternatives considered:**

- *`wantsNewTab` guard at every surface* — rejected: reimplements what
`RouterLink` already does; 10 copies of a five-way check to keep in sync.
- *CSS-grid tables* — rejected, see Scope.
- *`window.open()` on modifier* — rejected: popup-blocker bait, loses
background/foreground tab conventions, still no context menu or hover preview.
- *Anchor in every `<td>`* — rejected: 7 links per row in the a11y tree,
fragments row text.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Entity id / type* — already flow into `router.push`; trust unchanged. Path
safety comes from the **ID grammar**, not from encoding:
`internal/entity/id.go:134` pins every ID to `^[A-Za-z0-9][A-Za-z0-9_-]*$` and
`ValidateID` is documented as the single validity rule codebase-wide
(TKT-IZGF7T). `#`, `?`, `/`, spaces and unicode cannot occur, so
`encodeURIComponent` is a no-op here. Cite the grammar (RR-Z0AHAY) so a future
relaxation trips the right wire.
- *Column `link:` (`cellLink`)* — **verified closed allowlist in BOTH copies**:
Go `internal/dataentry/views_handler.go:713-725` and TS `EntityList.vue:475-483`
accept only `""`, `detail`, `document/*`; everything else returns `""`.
`javascript:`, `data:`, `//evil.com`, `JavaScript:` all hit the default branch.
**There is no new XSS sink** — an earlier draft claimed otherwise and was wrong
(RR-0PEI9F).
- No new network calls, auth changes, file access or crypto.

**Security-Sensitive Operations:**

- **`:href`/`:to` as a sink — defence-in-depth, NOT threat mitigation.** Per
CLAUDE.md's "write down which of the two you mean": this is **integrity /
invariant preservation, explicitly not confidentiality**. Config being
non-secret is irrelevant; the relevant fact is that editing `data-entry.yaml`
already requires repo write access. The guard exists so that if
`resolveLinkTarget`'s allowlist is later loosened, the render layer does not
silently become an XSS sink. Predicate is `/^\/(?!\/)/` — a single leading
slash, which **excludes protocol-relative `//evil.com`** (the plain
`startsWith('/')` in the earlier draft admitted it). A non-matching value
renders no anchor (click-only fallback).
- **Unit test pinning `resolveLinkTarget` returns `""` for unknown input** —
this is what actually protects the invariant; it fails if a passthrough branch
is ever added.
- **No ACL change.** A link is not an authorization decision: the target route
re-fetches through the same gated read path, and a hidden entity 404s
identically. Rows only render for entities the list already returned.
- All targets are same-origin SPA routes; no `target="_blank"`, no `rel`
handling needed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Level |
|---|---|---|
| 7 (and 1,5) | **`expect(router.resolve(anchorTo).fullPath).toBe(router.resolve(pushedTarget).fullPath)`** per surface — one invariant subsuming the scope/`from=search`/`edit_form`/`cellLink` string tests, and it stays true when a param is added later (RR-6PBTF1 / L1) | component |
| 1,2,3 | Playwright `context.waitForEvent('page')` after `click({ modifiers:['Meta'] })` and `{ button:'middle' }`; assert new-page URL AND that the original page URL is unchanged | e2e |
| 4 | assert `a[href]` present with expected resolved path per surface | component |
| 5 | existing `list.spec.ts:187` row-click test + component regression on push args | e2e + component |
| 6 | click checkbox → selection toggles, no nav; delete → dialog, no nav; kanban + relation-card drag-reorder still work; palette arrow-key roving focus intact | component + e2e |
| — | `resolveLinkTarget` returns `""` for unknown/scheme-bearing input (Go + TS) | unit |
| — | `shouldDeferToBrowser` truth table | unit |

Component tests mount a real router over `createMemoryHistory('/')`
(`StatusBar.test.ts` convention). E2E must add page-object methods, never inline
locators (`e2e/tests/AGENTS.md`, eslint-enforced).

**Edge Cases:**

- Empty/missing entity `type` → `entityDetailHref` returns `''` → render NO
anchor; plain click stays inert (never `/entity//id`).
- `cellLink` set → target is the cell link, not the entity route.
- `cellLink` malformed/server-supplied — the genuinely untested input (IDs are
grammar-constrained; `cellLink` is not). Assert no anchor rendered.
- RelationCards entry **not in `entityCache`** (`:181` no-ops today) → no anchor,
click still inert. No broken href.
- IssuesTable row with no entity (`canNavigate()` false) → plain `<span>`, no
role/tabindex.
- Kanban/RelationCards drag from the title → reorder works, no link-drag ghost.
- Modifier-click on checkbox cell / delete button → no tab, no navigation.
- Alt-click: on macOS Option-click *downloads* the target. Consistent with
`useDocumentClicks.ts:33`, which already includes `altKey`. Consequence noted
(RR-Z0AHAY M2).
- Right-click (`button === 2`) → no navigation, context menu shows.
- Palette modifier-click → tab opens AND palette stays open; plain click closes.

**Negative Tests:**

- Plain left-click must NOT open a tab.
- `defaultPrevented` event → no navigation.
- No `a[href=""]` rendered anywhere.
- No surface uses `target="_blank"`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| href/push divergence silently drops scope (RR-6PBTF1) | Single `entityTarget()`; the `resolve().fullPath` equality test |
| Overlay blocks text selection in rows | **Accepted and approved** — selecting text from list views is not a common action here |
| Overlay swallows nested controls | `select-cell`/`actions-cell` get `position:relative; z-index:1`; component tests click each |
| Nested `.stop` guards done imperatively — `handleDelete` calls `stopPropagation()` in JS (`:688`), invisible to a `.stop` grep | Enumerate controls per surface from the code survey, not by grepping |
| Anchors in drag sources break reorder (RR-NPDW9A) | `draggable="false"` on those anchors; drag tests for kanban + relation cards |
| `<a>` containing `<button>` = invalid HTML | Title-only anchors on mobile card, card-header, relation cards |
| Re-breaking TKT-IHC7C / RR-FC1B in EntityDetail | Anchor wraps text spans only; header handler untouched; re-read `:1017-1023` before editing |
| Anchor inside `role="option"` breaks listbox roving focus (RR-UHNHX4) | `tabindex="-1"` on the palette anchor |
| Row tab-stops go 0 → N per page (RR-Z0AHAY M4) | Deliberate: links should be reachable. One anchor per row, not per cell; row itself stays non-focusable |
| Link styling leaks into rows/cards | `color: inherit; text-decoration: none`; visual check light+dark |
| E2E new-tab flakiness | `context.waitForEvent('page')` with explicit timeout, assert on popup URL (available pre-load); no sleeps |

**Effort:** m — ten surfaces, but each is mechanical once `entityTarget()` and
the overlay exist. Test surface is the bulk of it.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — rows/cards are real links; cmd/ctrl/middle-click
opens a new tab.
- [x] ~~docs/metamodel.md, docs/cli-reference.md, README.md~~ (N/A: no
metamodel, command or project-level surface changes)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 6 responses. All resolved in this revision; the
revision also simplified the approach well beyond what the review asked.

| ID | Severity | Resolution |
|---|---|---|
| RR-6PBTF1 | critical | Shared `entityTarget()` is now mandatory; pinned by the `resolve().fullPath` equality test |
| RR-I4WQPX | critical | Incorrect DOM-event claim removed. Stretched-link overlay chosen (approved), so cmd-click works row-wide and AC 1 is honest |
| RR-NPDW9A | critical | `draggable="false"` on anchors in drag sources; drag tests for both kanban and relation cards |
| RR-0PEI9F | significant | Prose corrected (no new sink — closed allowlist verified in Go and TS); reframed as integrity defence-in-depth per CLAUDE.md; predicate tightened to `/^\/(?!\/)/`; pinning unit tests added |
| RR-UHNHX4 | significant | Both IssuesTable variants (desktop + **mobile**) in scope; role/tabindex/keydown removal specified; palette anchor gets `tabindex="-1"` |
| RR-Z0AHAY | minor | Cites `entity/id.go` `ValidateID` instead of encoding; helper renamed `shouldDeferToBrowser`; tab-stop 0→N called out as a deliberate decision |

**Rejected as incorrect:** the review claimed `EntityDetail.vue` does not import
`entityDetailHref`. It does — `EntityDetail.vue:16`, used at `:460`.
