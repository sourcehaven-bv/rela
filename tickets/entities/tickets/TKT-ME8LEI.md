---
id: TKT-ME8LEI
type: ticket
title: Migrate the data-entry SPA onto the rela-components library
kind: refactor
priority: medium
effort: xl
status: backlog
---

## Description

Rework the data-entry SPA onto the external `rela-components` library
(`Rl*`-prefixed), including its three-pane system, so rela's UI is assembled
from shared components rather than hand-rolled per surface.

The library is developed in a sibling repo
(`~/Work/sourcehaven/rela-components`) by a separate agent session. rela
consumes it through a **symlink** at `frontend/src/rl` during the migration;
vendoring it properly is deliberately deferred until the surfaces have settled.

Work happens on the `component-system-migration` branch. Detailed working notes
live in `.ignored/` (gitignored, not a deliverable):
`component-migration-notes.md`, `rltable-adapter-notes.md`,
`sidebar-handoff.md`.

## Done so far

Modals, buttons, menus, spinners, empty states, the app shell, keyboard-key
rendering, the entity list on `RlTable` (with an adapter mapping rela's
`ListColumn[]` onto the library's `TableColumn[]`), and the **sidebar** on
`RlSidebar` (528 lines down to ~131, plus a `sidebarNav.ts` mapping module).

Two live defects were fixed as a side effect. Adopting `RlTable` gave sorting a
keyboard route for the first time: rela's sort control was a bare `<th>` with a
click handler. And the sidebar migration surfaced BUG-15VIAN, a nav entry with
an unrecognised kind that rendered as a dead row.

## Remaining

- Remaining single-key `RlKbd` sites (pure re-skin, low value).
- Cleanup, then **vendor the library** and drop the symlink.

## Decisions taken

- **The library owns design decisions, Bootstrap-style.** A consumer imports
components and gets a good-looking, working app with the hard choices already
made, and that extends to layout components PLACING controls. Where rela and the
library disagree, take the library's default rather than overriding it locally.
- The sidebar footer, mobile-only before, shows on desktop too.
- The git branch/sync indicator is **dropped as a feature**. Check whether
`gitStore` retains another consumer before removing it.
- Theme switching is the library's: `RlThemeToggle` is three-state and
`RlSidebar` places it in the footer by default. rela passes no `#footer` slot
(that would replace the band including the toggle) and uses
`footer-label`/`footer-click` for Settings.
- rela's colour system is out of scope for now.

## Resolved along the way

- **No theme data migration was needed.** rela's `stores/ui.ts` already
carried `ThemeMode = 'light' | 'dark' | 'system'`, persisted under the `theme`
key, defaulting to `system`. The library's `ThemeChoice` is the same three
values, so `v-model:theme` binds straight on. The binary `toggleDarkMode` was
only the old button's entry point, never the storage.
- **Operator branding survived intact.** It was planned as reduced-and-
deferred, but keeping `class="sidebar"` on the `RlSidebar` root preserved every
hook: all 7 `customisation.spec.ts` tests pass unchanged. Keep that class.
- **rela's `system` mode has a JS dependency the library's does not.** rela's
`tokens.css` has zero `prefers-color-scheme` rules and keys dark off
`:root.dark` only, so the store's `matchMedia` listener is load-bearing — drop
it and rela's own chrome stays light on a dark OS while every `Rl*` component
goes dark. Mirroring the media query in `tokens.css` would remove the
dependency; that is a colour-system change, out of scope here.

## Constraints that must not be lost

- **Operator customisation is about the cascade layer.** The three
`expectSidebarBackground` assertions prove a low-specificity unlayered operator
rule beats rela's layered CSS, including a route chunk appended after
client-side navigation. Never "fix" a failure there by raising the operator
rule's specificity.
- **`NavItem.as` and the icon resolver are a security boundary.** Both carry
the component itself, never a name resolved at render time, so a config string
can never choose what gets mounted.

## Why a green build proves nothing here

Five times now a change typechecked, linted and passed the full unit suite while
being visibly broken. Verify in a browser, and hit-test interactive elements.
The common property is that the apparatus reported success while seeing nothing,
so the failure consumed the suspicion instead of raising it.

- Vue resolves **slot names at runtime**: a `cell-<key>` slot matching no
column renders empty, silently.
- Vue **silently drops an unknown boolean-`false` attribute**, so a prop the
component never declared typechecks clean and does nothing.
- happyDOM applies neither the library stylesheet nor media queries, so a
unit test cannot see `display: none`. Assert the class, not visible text.
- A sticky-header offset left the first table row visible, correctly marked
up and **unclickable**. Only `document.elementFromPoint` found it.
- The sidebar's collapse did nothing at all — the library emits
`toggle-collapse` and takes width from `--rl-sidebar-width`, and rela's old
collapsed-width rule had been dropped. 3131 unit tests passed over a rail that
never narrowed, because nothing in the suite could see a CSS variable that was
never rebound.

- The e2e suite serves the SPA bundle **embedded in the Go binary**
(`internal/dataentry/static/v2`), not the working tree. Running it without
`just build-server-e2e` first tests the last build: a new assertion failed
against code from twelve minutes earlier, and had it been written to pass
rather than to fail, the whole suite would have gone green over unbuilt
changes. Rebuild before believing an e2e result.

A related trap: a probe can lie in the same way. Measuring rows with
`document.elementFromPoint` reported "COVERED" for rows that were merely
scrolled off-screen, where it returns null — which nearly sent the sidebar bug
to the wrong diagnosis.

## Outcome: keyboard shortcut hints (1523c88b)

All 23 remaining bare `<kbd>` elements now render through `RlKbd`. The global
`kbd` chrome in `App.vue` and the seven local overrides that fought it are
deleted; the library owns the key's appearance.

Three things this surfaced that a pure re-skin would not have:

- `ActionConfig.key` is **optional**, so the bare element rendered an empty key
cap when an action declared none. `vue-tsc` caught it the moment the value
reached a typed prop. Guarded with `v-if`. The value is otherwise
well-constrained: `actionKeyRegex` (`internal/dataentryconfig/validate.go`) is
`^[a-z0-9]$`, so `RlKbd`'s split on `+` is a safe no-op here.
- The mobile hiding rule (`display: none` below the breakpoint) had to be
re-pointed at `.rl-kbd`, because `RlKbd` sets `display: inline-flex` in its own
scoped style and the rule has to beat it. This is the only genuinely
behavioural change in the batch and **no unit test can see it**, which is why
it now has an e2e test on the phone viewport.
- Two sites were hints in name only: SearchView wrote pairs as `<kbd>j</kbd> /
<kbd>k</kbd>` and DynamicForm crammed ⌘↵ into one cap. Both are now real
combinations, so they are announced as separate tokens rather than as one
unpronounceable string.

Three e2e tests added (324 → 327): the hint is announced, it trails rather than
leads its control (a leading hint would break every `name: /^Edit/` locator in
the suite), and it is hidden on a touch device.

## Outcome: detail panel beside the list (45639d89)

The first step toward the library's `pages--task-detail-beside-list` layout. A
plain row click opens the entity in `RlDetailPanel` beside the list rather than
replacing the page.

**A panel is not the canonical item view.** `/entity/:type/:id` stays the
address of the entity; `?selected=` on the list route addresses a row of *this
list*. The panel's expand control bridges between them. Keeping both is what
lets the existing navigation e2e tests stay meaningful, and matches the
library, whose `RlDetailPanel` ships a `variant="page"` and an `expand` action
for exactly this split.

`?selected=` is the **only** state — there is no local ref mirroring it. That
is what makes a shared link, a reload and a back step all restore the panel
through one code path. A mirrored ref would need the echo-signature
reconciliation `useUrlFilterSync` carries; with no local copy there is no echo.
The entity *type* is deliberately not in the query: it is `listConfig.entity`,
so putting it in the URL would let a link name a row the list cannot contain.

### Two traps

- **`RouterLink` ignores `preventDefault`.** A plain `@click` handler that
calls `preventDefault` still navigates, because RouterLink's own listener
checks its guard conditions rather than `defaultPrevented`. The click has to be
taken on the **capture** phase with `stopPropagation`. The first e2e run caught
this: the panel assertion failed because the click had gone to the entity page.
Only plain clicks are taken; cmd/ctrl/shift/middle stay the anchor's own
default, so "open in new tab" still reaches the entity's own page.

- **`selected-id` was carrying the wrong state.** `EntityList` passed the j/k
cursor into RlTable's `selected-id`, which the library documents as "the row
whose detail is open". Harmless until a panel existed; two distinct states
after. Reassigning it to the panel left the cursor unmarked and broke
`list.spec.ts`'s keyboard test — the test was right, the prop was overloaded.

The cursor now uses RlTable's `cursor-id`, requested from and delivered by the
component repo during this work. Worth recording *why* the library owns it:
`RlTableRow` already spends `aria-current` on the open row and `aria-selected`
on the checkbox, so a third state announced from rela would have been a second
`aria-current` in one row. The library's answer is
`aria-activedescendant` on the grid, because the cursor is a focus position
rather than a property of the row. rela paints nothing and moves the index only.

### Fallout worth knowing

`.empty-state` is a generic class, and the panel renders one of its own
(`DocumentsPanel`, for an entity with no document content) — so the list page
object's "rows or empty state" wait became ambiguous and failed under strict
mode. Scoped to `.entity-list .empty-state`.

**Not a library naming problem**, though it was first reported to the component
repo as one. The library's class is `rl-empty-state`; both colliding elements
are rela's own unprefixed `.empty-state`, of which there are six across the
SPA. Nor would more prefixing help the general case: once two panes are on
screen, `rl-table-row--selected` legitimately appears twice, each table marking
its own open row. The ambiguity is **per-instance, not per-name**, so the fix
is to address the instance — `row-attrs` is the library's hook for exactly
that, and is what the cursor stopgap had already used. Expect more of these as
the three-pane layout lands, and scope the locator rather than renaming
anything.

Six e2e tests added (327 → 333), including cursor-independence: open the panel,
press j twice, assert the panel has not moved.

## Outcome: page header hoisted into the shell (42b54256)

The header band is rendered by `RlAppShell`'s `#header` slot, so it spans the
content and the detail panel both. Left in the view it sits inside the content
pane and stops at the panel's left edge, which visually cuts the page title in
half the moment a panel opens.

`usePageHeader` is the outlet, built the same way as `useDetailPanel` and for
the same reason: the slot is two levels above the routed view that knows what
belongs in it, and the shell's element and the view mount in an order Teleport
cannot be told to wait for.

### It carries slots, not props

The one real difference from `useDetailPanel`. A header's tools are the view's
own live controls — the search box and the filter menu that its keyboard
shortcuts hold template refs to (`onFocusSearch`, `onOpenFilter`). A slot
renders in the scope that DECLARED it, so those refs keep resolving from the
view after the markup moves two levels up. Passing a component and props would
have meant marshalling refs across the boundary and re-binding every handler.

`PageHeaderContent` is the write side: a view declares its header as ordinary
markup and the component renders nothing where it sits. It is a `.ts` render
function rather than an SFC because an SFC with an empty root is not valid Vue.

### Two traps

**A second hamburger.** `RlPageHeader` renders its own nav toggle, so a screen
with a hoisted header had two — the library's and App.vue's `.mobile-menu-btn`.
App.vue now suppresses its own when the outlet is filled (gate 3 on
`showHamburger`). The mobile page object locates either, since the guarantee is
"a drawer trigger is reachable", not which element serves it.

**Two gutters.** The band pads itself with `--rl-page-gutter`; rela's main pane
hard-coded 24px. Different numbers, so the list content could not line up with
the title above it. The pane now takes the library's token, and the 768/480
breakpoints no longer restate horizontal padding — the token carries its own
breakpoints and the safe-area inset. This changed the desktop gutter 24px → 32px.

`mobile-layout.spec.ts`'s padding-contract test failed on that: it read
`--page-padding-x` with `getPropertyValue`, which returns a custom property
UNRESOLVED, and the value is now `calc(16px + 0px)` rather than a bare `16px`.
The used length was correct throughout. The helper resolves it by laying the
value out as a real width, which is what the bleeding bars actually get.

### Testing

Five e2e tests (332 → 337). They are geometry assertions on purpose: happy-dom
has no layout engine and reports the same box for a header that spans both
panes and one that does not, so a unit test cannot tell them apart. All five
were mutation-verified — renaming the `#header` slot fails every one.

Unit tests that mount a view alone now render no header, so its search box is
absent and an assertion about it reads as "search is hidden". `withPageHeader`
(`composables/pageHeaderTestHost.ts`) mounts the view together with the outlet,
restoring the half of the app that paints it. Six tests across
`EntityList.test.ts` and `EntityList.world.test.ts` go through it.

### Still in the view

The kanban, calendar and gantt views keep their own in-view headers, as do
dashboard, search and settings. Each draws a different one (the calendar has a
period navigator, the gantt a zoom control), so they are separate pieces of
work rather than one sweep. A screen that sets no header renders no band at
all, which is what lets them stay as they are.

## Outcome: one theme, the library's (27221ab5)

rela's own palette is gone. Every rule in the SPA reads `--rl-*`; 1289
replacements across 76 files, and `src/styles/tokens.css` no longer declares a
colour of its own.

### Why it had to go

The two palettes could not be kept in step. With `.dark` on the html element,
`--rl-color-bg` still resolved to `#fff` while rela's `--bg-color` was dark, so
the app shell painted white on a dark page — light and dark visibly on screen
at once.

### The carve-out recognised the wrong thing

`relaCssLayer.ts` lifts the token contract out of `@layer rela` so an
operator's unlayered `custom.css` can still override it. It matched a palette
by SELECTOR — `:root` and `:root.dark`. rela-components selects its dark
palette two other ways: a bare `.dark` (which also themes a subtree, not just
the document) and `:root:not(.light)` inside `@media (prefers-color-scheme:
dark)`. Neither matched, so the dark tokens stayed layered while the light ones
were lifted out, and an unlayered declaration beats a layered one at ANY
specificity. The light palette won regardless of the class. This looks exactly
like a JavaScript bug and is not one.

It now recognises a palette by what it DECLARES — custom properties, plus
`color-scheme`, which picks the built-in treatment for form controls and
scrollbars and so belongs with the palette that decides the theme.

That alone was not enough. lightningcss MERGES adjacent rules sharing a
selector, and `dark.css` declares `.dark` twice: the palette, then a paint rule
giving a dark subtree its own background. Minified they become one rule holding
both, which "carve out only pure palettes" rejects — re-layering the whole dark
palette. A mixed rule is now SPLIT rather than rejected, so neither half
depends on how the source happens to be authored or how hard it is minified.

### Emptying tokens.css broke custom apps

`src/styles/tokens.css` is embedded byte-for-byte as
`internal/dataentry/apps_tokens.css` and served to app iframes as `_rela.css`.
Emptying it left custom apps AND the embedded markdown editor with no colour
tokens at all. This surfaced as an e2e failure in the editor's toolbar
(`apps.spec.ts`, "disables a command that would do nothing"), which failed 2
runs in 3 — a flaky-looking failure with a deterministic cause.

An `@import` cannot fix it: `_rela.css` is the whole contract and is served
cross-origin. So the palette is COPIED, and `scripts/gen-app-tokens.mjs`
generates the copy (`npm run gen:tokens`) rather than leaving it to be
hand-maintained, which would be the same drift with an extra step. It fails
loudly when it finds no palette rules, because a silent empty palette is how
the editor lost its colours the first time.

`TestAppCSSSource` asserted `:root.dark`; the dark selector is a bare `.dark`
now. The app SDK already toggles that exact class on `<html>`, so the mechanism
was never broken — only the assertion.

### Parked here, not done

`dataentryconfig.deriveTheme` still emits rela's retired token names from its 8
base colours, so a palette-configured project now covers none of the library's
tokens. `TestPaletteCarriesEveryDefaultToken` is `t.Skip`ped naming this
ticket. The test is unchanged and the property it guards is still right — an
operator palette must not blank out tokens — what is missing is the port.

`applyPalette` in `stores/ui.ts` writes palette values as inline styles on
`<html>`, which beat any stylesheet, so it needs porting with it.

## Outcome: the status bar is gone (27221ab5)

The 24px strip pinned across the bottom of the viewport is removed. It predated
the app shell and fought it: it painted over the shell's own bottom edge, and
its theme toggle and Settings link duplicated the ones `RlSidebar` already
rendered — two System/Light/Dark controls visible at once.

Everything it carried is in the sidebar's `#footer` slot now
(`components/common/SidebarFooter.vue`): git status, the next-action chip,
Settings, About, Shortcuts. The theme picker is NOT duplicated there —
`Sidebar.vue` owns the binding and renders the library's control beside the
footer component, because replacing the slot replaces the library's default
toggle along with its default link.

Both overlays stay teleported: the sidebar is a scroll container and a drawer
on narrow viewports, so a popover anchored inside it would clip.

`RlSidebar`'s `theme-toggle`, `footer-label`, `v-model:theme` and
`@footer-click` are inert once the slot is supplied, so they are gone from the
call site.

`status-bar.spec.ts` becomes `chrome-footer.spec.ts`, named for what the band
holds rather than where it sits. It gains a test asserting exactly ONE theme
picker is on screen — the duplication was the defect. `theming.spec.ts` no
longer clicks an implied two-state toggle: the library's control is a three-way
radiogroup, so each test names the state it wants, and "click twice and expect
to be back" could not express `system` at all.

## Deferred: operator extensibility, after the storyboard match

The order is deliberate: get rela looking exactly like the storyboards on the
library's components first, then restore extensibility. So the following are
known, accepted gaps rather than oversights, and none of them is a reason to
keep a private copy of a library decision.

**`custom.css` hooks now sit on library components.** `docs/data-entry.md`
promises one `custom.js` / `custom.css` serves the next-action UI wherever it
renders, and the two surfaces have diverged: the page-level `NextActionCard`
still puts `rela-na-band` / `rela-na-message` on plain elements, while the
sidebar footer's popover puts them on `RlTag` and `RlText`. An operator rule
still WINS the cascade (theirs is unlayered, rela's is layered), so this is not
a broken override. What it breaks is uniformity — a rule written against the
page surface may fight a library component's own scoped layout in the footer.
Decide per hook whether the operator contract belongs on rela markup or whether
the library component should carry it, and reconcile both surfaces together.

**The operator palette still speaks the retired token names.** Carried over
from the single-theme work above: `dataentryconfig.deriveTheme` and
`applyPalette` in `stores/ui.ts` both need porting to `--rl-*`.
`TestPaletteCarriesEveryDefaultToken` is skipped naming this ticket until they
are.

## Outcome: rela's global component CSS is gone (50db1a07)

`App.vue`'s style block lost 240 lines. The shared utility classes it carried
are deleted outright: `.btn` and its five variants (dead — a precise grep for
`class="btn"` matched nothing, where a loose `\bbtn\b` had made it look like it
had 19 users), `.modal` and friends, `.page-header`, `.header-actions`.
`DuplicateModal` was the last consumer of the global modal system and moved to
`RlModal`, which also let its own hand-rolled focus capture, focus restore,
Escape handling and title id go — four behaviours the library already owns.

**The library was asked for what was missing, and rela was left plain in the
meantime.** Two gaps went to the component repo rather than being papered over
locally: a centred region that REPLACES a pane while it loads or after it fails
(`.loading-state` / `.error-state`, 15 files, and neither `RlEmptyState` nor
`RlBanner` means that), and a generic anchored popover for prose plus mixed
controls (`RlMenu` forces `role="menu"` and closes on any click inside, so a
link in the body is dismissed before it navigates). Both shipped as
`RlStatusRegion` and `RlPopover` and are now wired up.

**A library API gap found by using it.** `RlBackButton` took only `href`, so a
router-based app had to resolve the URL itself — which makes the consuming
component depend on a router that can `resolve`, and breaks every test that
mocks `vue-router` with a partial `{ push, replace }`. That is 11 test files
here. It now takes `as`, matching `RlButton`. The first fix still dropped the
href: binding `:href` unconditionally passes an explicit `undefined`, which
beats what RouterLink resolved internally, so the anchor rendered with no URL
and lost middle-click. Only a REAL router reproduces it — a stub that takes
`href` by fallthrough loses to an explicit binding and renders correctly no
matter what the parent passes.

## Known unrelated flake

`src/components/entity/CopyMenu.test.ts > closes the menu after a choice` fails
roughly two runs in three in isolation, order-independent, and predates this
work. Confirmed not caused by this migration by stashing the changes and
reproducing the failure on a clean tree.
