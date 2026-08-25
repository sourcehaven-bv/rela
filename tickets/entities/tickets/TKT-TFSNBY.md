---
id: TKT-TFSNBY
type: ticket
title: Framework-level loading/pending indicator system for the data-entry SPA
kind: enhancement
priority: medium
effort: l
tags: needs-design
status: done
---

## Problem

Loading indicators in the data-entry SPA are ad-hoc and janky on fast
connections. Indicators appear at 0ms and flash for a few frames when a request
resolves in 40ms; full-block spinners collapse and re-expand page regions;
button labels swap between strings of different widths so the control resizes
under the cursor mid-click.

The operating model is the key constraint: rela is normally a local or
well-connected server with tiny per-request cost, so **the common case is well
under 100ms and the occasional slow case is 1-2s**. Anything beyond that is a
broken connection, not a slow one, and is an error condition rather than a
loading state. Published design-system delays (Spectrum's 1000ms, Primer's
1000ms) are tuned for public web apps on bad mobile networks and are wrong here:
at a 1-2s worst case, a 1000ms delay plus a minimum-duration hold means the
indicator routinely outlives the work it describes.

## Design (settled)

Three indicators, one per class, with no overlap. The class is decided by **who
started the operation and whether the user needs to know it finished** — not by
whether it blocks.

| Class | Indicator | Delay | Min duration |
|---|---|---|---|
| Navigation (route change, cold data load) | Global top bar | 250ms | 300ms |
| Explicit action (Save, Create, Delete, Run) | Label swap on the triggering control | 500ms | 400ms |
| Ambient (autosave) | Fixed status region | 0ms* | 600ms |
| Background revalidation (SSE refetch) | **nothing** | — | — |

\* The ambient 0ms is measured from *request start*, which `useAutoSave` has
already deferred by an 800ms debounce. **The debounce is the ambient class's
effective delay** — see RR-ZT9DXG and the plan's Approach §6. Do not "fix" the
apparent inconsistency by adding an entry delay.

**Delay scales with how invasive the indicator is, not with how slow the request
is.** A top bar fading in peripherally is nearly free and can sit at 250ms. A
button mutating under the cursor is foveal and highly invasive, so it waits
500ms. This is why the published ~1000ms figures exist at all — they are
calibrated for a *spinner* on a button, the most invasive combination.

### Governing rule

> If an operation completes before its indicator's delay elapses,
> nothing is ever shown.

Under the operating model above this means the app shows **no loading UI at
all** in the common case. "Silent" extends further for invasive indicators than
for gentle ones: a 600ms save shows nothing, a 600ms navigation shows a soft
bar. Both are correct.

### Explicit actions: label swap, not spinner

Only one button treatment, and it is the text swap ("Save" -> "Saving…"). Chosen
over an in-button spinner because:

- It says what is happening; a spinner says only that something is.
- It is the only option that survives `prefers-reduced-motion` — text has
no motion to suppress. (The SPA currently honours reduced-motion in 1 of 11
animation sites, so choosing the treatment that does not need the rule is a real
simplification.)
- It is accessible for free: "Saving…" *is* the announcement. A spinner
needs a parallel visually-hidden live region to say the same thing, so the
spinner path is strictly more code conveying strictly less.
- It is the framework-native answer elsewhere (Turbo ships it as
`data-turbo-submits-with`).

Cost: the pending label is verb-specific (Save/Saving, Create/Creating,
Delete/Deleting). Make it a **required explicit prop**, never derived by string
munging from the resting label — that breaks on the first irregular verb or
translated string. **6 call sites** (was estimated at ~12 before `ConfirmModal`
was excluded — see below).

Icon-only controls (DocumentView / DocumentsPanel refresh) swap the icon in
place in the same box. Narrow exception, not a second tier.

**`ConfirmModal` is a sanctioned exception** (RR-VFI1W0). It appends one
character to a caller-supplied `confirmLabel` rather than swapping verbs, and
that label is optional with a `'Confirm'` default — so migrating it would push a
new required prop through every `useConfirm()` caller to replace behaviour that
is already minimal-shift and already uses U+2026. It keeps its current
treatment.

### No layout shift, at two levels

1. **Per button** — both label states render into one CSS grid cell
(`grid-area: 1/1`), the inactive one held by `visibility: hidden` so it still
contributes its box. The cell sizes to `max(width("Save"), width("Saving…"))`;
the swap causes zero reflow. Browser-computed, so it is correct after font load
and in every language. Rejected: `min-width` (hand-tuned per button, breaks on
translation), JS measurement (layout thrash on mount, stale after webfont swap),
`ch` units (fails on non-Latin scripts). **Do not scale the font to fit** —
shrinking type mid-interaction is itself a visible mutation and reads as broken.

2. **Per action group** — paired footer buttons (Save+Cancel,
Create+Cancel) get equal width via `grid-auto-flow: column; grid-auto-columns:
1fr`. Without this the width reservation makes a lone "Save" sit in a visibly
over-padded box next to a snugly-fitted "Cancel" — a static asymmetry on every
form. Only for genuine pairs, not toolbars, and not when one label is far longer
("Delete permanently" would stretch Cancel absurdly).

The top bar is absolutely positioned, so per the CLS definition it cannot shift
other content — layout-stable by construction.

### Keep-previous-data beats every indicator

For "load the next set" and entity-to-entity navigation the correct indicator is
often *none*: hold current content on screen until the new data resolves rather
than replacing it with a spinner. This is a data-layer setting (Pinia Colada
`placeholderData`), not a visual one, and it removes the worst layout shift in
the app (a full table collapsing to a ~140px centered spinner and re-expanding).

`DocumentView.vue:168` and `DocumentsPanel.vue:189` already implement the
hand-rolled equivalent (`v-if="loading && !docContent"`) and are the in-repo
precedent — **preserve that guard**, do not mechanically replace it (RR-TCZWUI).

### Edge rules

1. **Exactly one indicator per user act.** A save shows the button state,
never the bar. If an action triggers navigation (Create-then-redirect), the
button owns it through the save and the bar takes over at the route change —
sequential, never concurrent.
2. **The bar is for *new* content, not refreshed content.** Gate on "no
data yet" (`isPending`), not "a request is in flight" (`isFetching`).
3. **Overlapping operations must not strand the indicator.** For
*navigation* specifically this means identity-tracking, **not** reference
counting: navigations supersede each other, and a `beforeEach`-increment /
`afterEach`-decrement counter leaks permanently on the cancelled/aborted
navigations that BUG-6C3V documents as routine (RR-B7U3I8 — critical; see the
plan's Approach §2). Reference counting remains correct for a concurrent
in-flight *query* count, should that later feed the same bar.
4. **Ambient never escalates.** Autosave failure becomes a persistent error
glyph in the same fixed region — not a toast, bar, or button state.
5. **Broken connection is an error, not a loading state.** A request past a
ceiling (~10s) fails and surfaces where errors already surface. No escalation
ladder. Separately, `useEvents` already tracks SSE disconnection with backoff
but renders it nowhere; surfacing that in the status bar is the honest answer to
"the connection is down" and keeps the three pending indicators from having to
cover it.
6. **`aria-disabled` applies to primary actions only** (RR-R5VL59).
Secondary/Cancel buttons keep native `disabled`, because `aria-disabled` leaves
a control activatable and "cancel an in-flight confirm" has no defined meaning.
Handler suppression must cover keyboard activation as well as click.

### Explicitly out

- **No skeletons.** They belong in the 1-10s band and never earn their
place at a 1-2s worst case. Evidence is also against them: the Viget study
(n=136, identical durations) found skeletons lost to both spinners and blank
space on every metric — 59% vs 74% "loaded quickly", 2.82s vs 2.41s perceived
wait.
- **No determinate progress bars** — same band argument.
- **No in-button spinners** (see above). The only spinner that survives is
the existing ambient autosave glyph.

## Scope

In scope: the three indicators, one shared delay/min-duration gate, motion
tokens, and migrating existing ad-hoc sites onto them.

Out of scope: finishing the Pinia Colada migration (FEAT-XY2D1L) — this ticket
must work with both the Colada views and the legacy store-based ones. Where
keep-previous-data needs Colada, that dependency is noted rather than absorbed.
Also out: `ConfirmModal` (above), retrofitting `useAutoSave` onto the shared
gate, and surfacing SSE disconnection.

## Notes for planning

- `useAutoSave.ts` already implements half the gate: `MIN_SAVING_VISIBLE_MS
= 600` and `SAVED_INDICATOR_MS = 1200`, with a real hold timer.
`AutoSaveIndicator.vue` is already correct — fixed 28px box, opacity fades,
reduced-motion, `role="status"` live region. **Generalize from these; do not
rebuild them.** The ambient indicator keeps its current behaviour and becomes
the third citizen of the new system.
- The shared gate is a four-state machine (`IDLE -> DELAY -> DISPLAY ->
EXPIRE`), per the `spin-delay` reference implementation. No framework surveyed
(SWR, TanStack Query, SvelteKit, htmx, Pinia Colada) ships a delay threshold —
all expose instantaneous booleans, so this is necessarily userland code.
TanStack **Router** is the only library with the full API, and its `pendingMs` /
`pendingMinMs` naming is the shape to copy.
- Timings belong in `scales.css` (SPA-only), **not** `tokens.css`, which is
a byte-identical colour-only contract with the Go binary.
- `aria-busy` currently appears nowhere in the codebase; it is the natural
CSS hook for pending states (Turbo applies it automatically; htmx uses a
`.htmx-request` class for the same purpose).
- Add a global `prefers-reduced-motion` rule while here — 10 of 11
`animation: spin` sites currently ignore it.
- Use `…` (U+2026) to match `ConfirmModal` and `AutoSaveIndicator`; the
current Save buttons use `...`.

### Open question — RESOLVED in planning

`EntityList.vue` set `placeholderData: (prev) => prev` (:370) but computed
`loading` from `isPending` (:85), with contradictory comments about which wins
on a page change. **Settled against Pinia Colada 1.4.2 source:** when
placeholder data is active Colada overrides the exposed state to `{status:
'success', data: placeholderData}` (`index.mjs:711-715`), so `isPending` is
false during a param change and **pagination does not flash the spinner**. The
`:361` comment was right; `:84` was stale. Now pinned by two tests
(mutation-verified) and the stale comment is corrected.
