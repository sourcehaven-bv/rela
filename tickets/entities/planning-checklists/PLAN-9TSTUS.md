---
id: PLAN-9TSTUS
type: planning-checklist
title: 'Planning: Framework-level loading/pending indicator system for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
1. `useDelayedPending` — one shared four-state gate (`IDLE → DELAY → DISPLAY → EXPIRE`) with `delay` / `minDuration` options.
2. `<ActivityBar>` — global, absolutely positioned navigation indicator.
3. `<PendingButton>` — label-swap pending state with two-level width reservation.
4. Motion/timing tokens in `scales.css` + a global `prefers-reduced-motion` rule.
5. Migrating the ad-hoc spinner sites and label-swap buttons onto the above.

OUT:
- Finishing the Pinia Colada migration (FEAT-XY2D1L). This must work with BOTH Colada views (EntityList, KanbanView) and the ~20 legacy `ref(false)` sites.
- Skeletons, determinate progress bars, in-button spinners (design-rejected, see ticket).
- Rendering SSE disconnection state in the status bar. Named in ticket edge rule 5 as the honest home for "connection is down", but it is a separate visible feature with its own UX; filing separately keeps this ticket to the pending-indicator framework.
- Route-level code-split chunk loading feedback beyond what `<ActivityBar>` gives for free.
- **`ConfirmModal` (per RR-VFI1W0)** — stays on its append-ellipsis behaviour. See Approach §3a.
- **Retrofitting `useAutoSave` onto the shared gate** — it has extra states (`saved`, `error`) and equivalent minimum-duration semantics already.

**Acceptance Criteria:**

1. **Sub-delay operations show nothing.** A save resolving in 100ms never changes the button; a navigation resolving in 100ms never paints the bar.
*Test:* fake timers, resolve at 100ms, assert no DOM mutation on the control and
`.activity-bar` absent throughout.
2. **Supra-delay operations show, and hold for the minimum.** A save resolving at 700ms swaps the label at 500ms and keeps it until at least 900ms.
*Test:* fake timers; assert label at t=500, still pending at t=750, restored
after t=900.
3. **Button width never changes between states.**
*Test:* jsdom lacks layout, so assert the CSS contract structurally — both label
spans present, inactive one carrying `visibility: hidden`, neither `display:
none` nor `v-if`. Real geometry covered by an e2e
`getBoundingClientRect().width` assertion.
4. **Paired action buttons are equal width.**
*Test:* structural (group carries the equal-width grid class); e2e asserts equal
measured widths.
5. **A superseded navigation never strands the bar.** (Revised per RR-B7U3I8.) Navigate A→B, then B→C before B settles: the bar hides when C settles, and is not left displayed.
*Test:* drive the real router through overlapping pushes including a `cancelled`
navigation failure; assert the bar clears. **Plus an explicit leak test**: force
a cancelled/aborted navigation and assert the bar is not displayed afterwards.
6. **Background SSE revalidation shows nothing.**
*Test:* extend the existing `useEvents` invalidation tests — assert
`.activity-bar` and `.loading-state` both absent across the refetch.
7. **Pagination holds previous rows.** Covered and pinned — see Research.
8. **Reduced motion.** Under `prefers-reduced-motion: reduce` no `animation` remains on any pending affordance, but the *state* is still conveyed (the text swap still happens).
*Test:* CSS-source assertion, in the spirit of the existing `relaCssLayer`
tests.
9. **Accessibility.** The primary pending control carries `aria-disabled="true"` (not `disabled`), its handler is suppressed for **both pointer and keyboard activation**, and the state is announced once via a polite live region.
*Test:* assert the attribute; assert neither `click` nor
`keydown.enter`/`keydown.space` fires the handler; assert live-region text
content.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the survey was done conversationally before the ticket
was filed and its conclusions are recorded in the ticket body. A RES entity
would duplicate it.

**Existing Solutions:**

*Libraries considered:*
- `spin-delay` (defaults `delay: 500`, `minDuration: 200`) — the reference four-state implementation. React-only; port the state machine, do not add the dep.
- `nprogress` — no built-in delay, starts immediately. Rejected: we would have to add the gate anyway, and it brings its own DOM/CSS.
- TanStack **Router** — the only library shipping the full API (`defaultPendingMs: 1000`, `defaultPendingMinMs: 500`). Not adoptable (we use vue-router) but **its option naming is the shape to copy.**
- Verified that **no data library ships a delay threshold** — SWR, TanStack Query, SvelteKit, htmx and Pinia Colada all expose instantaneous booleans. This is necessarily userland code.

*Similar patterns in codebase:*
- `useAutoSave.ts:40` `MIN_SAVING_VISIBLE_MS = 600`, `:36` `SAVED_INDICATOR_MS = 1200`, hold timer at `:199-209`. **Half the gate already.**
- `AutoSaveIndicator.vue` — already correct: fixed 28px box, opacity fade (not `display`), reduced-motion, `role="status"` live region with empty-at-idle text. The template to generalize from.
- **`DocumentView.vue:168` / `DocumentsPanel.vue:189` — `v-if="loading && !docContent"`.** The in-repo precedent for keep-previous-content, hand-rolled. Same semantic as Colada's `isPending`.
- `usePageData.ts:14-18` — shared async lifecycle helper that owns no loading state; its own header says it should be replaced by a `useAsyncState`.
- `EntityList.vue:370` `placeholderData: (prev) => prev`.

*Prior art in the graph:* `FEAT-XY2D1L` (dependency, not scope), `TKT-U62DVR`
(established the ambient behaviour this preserves), `TKT-8VVBRI` (the
tokens/scales split), `RR-R51D0` ("Don't blank results during in-flight
refetch") — the same defect class fixed pointwise before; this generalizes it.

**RESOLVED — the ticket's open question.**

Verified against Pinia Colada 1.4.2 source, not inferred:
- `index.d.mts:748` — `isPending` is *"whether the request is still pending its first call. Alias for `status.value === 'pending'`"*.
- `index.mjs:711-715` — when placeholder data is active the exposed `state` is **overridden** to `{status: 'success', data: placeholderData, error: null}`.
- `index.mjs:371` — a new entry seeds `placeholderData` from the previous entry, chaining across consecutive page changes.
- `index.mjs:466` — the placeholder is cleared once the entry leaves `pending`.

Therefore `isPending` is **false** during a param change whenever previous data
exists: `EntityList.vue:361`'s comment was right, `:84`'s was **stale**.
**Pagination does not flash the spinner today.**

Actions taken during planning: added the `pagination keeps previous rows` tests;
**mutation-tested** the guard (removing `placeholderData` fails the intended
assertion); corrected the stale comment. Full suite green (113 files / 1826
tests).

Colada also exposes `isPlaceholderData` (`index.d.mts:752`) — useful for
disabling Next during a page change.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*1. `src/composables/useDelayedPending.ts`* — the shared gate.

```ts
useDelayedPending(source: Ref<boolean> | (() => boolean),
                  opts?: { delay?: number; minDuration?: number }): Ref<boolean>
```

Four states. `IDLE`: nothing shown. `DELAY`: source true, timer armed, still
false. `DISPLAY`: timer fired, true, minimum armed. `EXPIRE`: source false but
minimum unmet — stay true until it elapses. Source going false during `DELAY`
returns straight to `IDLE` and **shows nothing** (the governing rule). All
timers cleared on unmount. Non-finite/negative options coerce to defaults.

*2. Navigation pending — REVISED per RR-B7U3I8 (critical).*

The original counter design leaked. `router.onError` (`router/index.ts:240-248`)
**early-returns** on `cancelled`/`aborted`/`duplicated` failures and on
`isCancelledFetch` — exactly the cases where `afterEach` also does not fire, and
which BUG-6C3V documents as routine in Firefox across the 15 lazy `() =>
import()` routes. A `beforeEach`-increment / `afterEach`-decrement pair would
therefore strand the bar **permanently**, which is worse than the flashing this
ticket fixes.

**Adopt option (a): track the pending navigation by identity, not by count.**

```ts
const pendingTarget = shallowRef<RouteLocationNormalized | null>(null)
router.beforeEach((to) => { pendingTarget.value = to })      // overwrites; cannot accumulate
router.afterEach(() => { pendingTarget.value = null })
router.onError(() => { pendingTarget.value = null })          // unconditional, BEFORE the existing early-returns
const navPending = computed(() => pendingTarget.value !== null)
```

A superseded navigation overwrites rather than stacking, so a lost one cannot
leave a residue — the leak is impossible by construction rather than by careful
bookkeeping. The `onError` clear must be registered as a **separate
`router.onError` call placed before the existing handler**, so the existing
early-returns cannot skip it; do not edit the existing handler's control flow.

This deliberately **narrows ticket edge rule 3**: navigations *supersede* each
other (the latest is the only one that matters), so reference counting is wrong
for them. Reference counting still applies to the in-flight *query* count if
that is later folded into the global signal. AC5 is restated accordingly and
gains an explicit leak test.

`<ActivityBar>` in `App.vue` outside the routed area: `position: fixed; top: 0`,
2-3px, `transform: scaleX()`, opacity fade ~200ms, `pointer-events: none`,
`aria-hidden="true"`. Consumes the gate at 250/300.

*3. `src/components/common/PendingButton.vue`*

```vue
<PendingButton :pending="saving" label="Save" pending-label="Saving…" @click="..." />
```

Both labels always in the DOM, stacked in one grid cell (`display: grid`, both
children `grid-area: 1/1`), inactive one `visibility: hidden`. Gate at 500/400.
`pendingLabel` is **required** — never derived.

*3a. `aria-disabled` scope — REVISED per RR-R5VL59 (significant).*

`aria-disabled` applies to the **primary action button only**. Secondary/Cancel
buttons keep native `disabled`. Rationale: `aria-disabled` keeps the control
focusable *and activatable*, so every such button needs defined in-flight
semantics. For a primary action "ignore the second activation" is correct and
sufficient. For Cancel it is not — `useConfirm.ts:89-96` holds `busy` across the
awaited `onConfirm`, and there is no defined meaning for cancelling an in-flight
confirm (abort? ignore? close and let the write land?). Defining that is out of
scope, so Cancel keeps native `disabled` and its existing focus behaviour
(`ConfirmModal.vue:55-60` focuses Cancel on open — a WAI-ARIA dialog requirement
that must not regress).

Handler suppression must cover **keyboard as well as pointer** activation, since
keyboard reachability is the whole point of `aria-disabled`. For a destructive
confirm a second activation is a second DELETE, so AC9 tests both paths.

*3b. `ConfirmModal` is a sanctioned exception — per RR-VFI1W0 (significant).*

It is **removed from the migration list**. It does not swap verbs; it appends
one character (`ConfirmModal.vue:48-49`, `confirmLabel + U+2026`), and
`confirmLabel` is an optional prop defaulting to `'Confirm'`
(`useConfirm.ts:34,59`). Migrating it would force a new required `pendingLabel`
through every `useConfirm()` caller to replace behaviour that is already
minimal-shift and already uses the U+2026 this ticket standardises on. Document
it in `frontend/CLAUDE.md` as the one sanctioned non-`PendingButton` pending
affordance.

*4. Action-group width* — `.form-actions { display: grid; grid-auto-flow:
column; grid-auto-columns: 1fr; justify-content: end }`. Genuine
Save/Cancel-style pairs only; not toolbars, not far-longer labels.

*5. Tokens* — in `scales.css` (NOT `tokens.css`, the byte-identical colour-only
Go contract): `--pending-delay-nav: 250ms`, `--pending-min-nav: 300ms`,
`--pending-delay-action: 500ms`, `--pending-min-action: 400ms`, `--pending-fade:
200ms`. One JS module mirrors these; CSS reads the tokens for transitions.

*6. Ambient class — clarified per RR-ZT9DXG (significant).*

The ambient indicator's "0ms delay" is measured from **request start**, which
`useAutoSave.ts:144-148` has already deferred by `baseDebounceMs = 800`. **The
debounce IS the ambient class's delay** — it filters out fast/transient work
before a request exists, which is why no additional entry delay is needed and
why the class is not inconsistent with the governing rule. This must be stated
in `frontend/CLAUDE.md`, otherwise a future reader "fixes" the apparent
inconsistency by adding a delay and makes autosave feel broken. The ticket's
stated rationale ("so the user perceives a smooth idle → saving → saved
transition") is corrected: the user perceives ~800ms of quiet, then the
indicator. Re-check the 600ms minimum against that during implementation.

*Alternatives rejected:* in-button spinner (dies under reduced motion; needs a
parallel live region to say less); `min-width` / JS measurement / `ch` units;
font-scaling (a visible mutation in itself); skeletons (wrong duration band; the
Viget study found them losing to both spinners and blank space).

**Files to modify:**

New: `useDelayedPending.ts`, `useNavigationPending.ts`, `ActivityBar.vue`,
`PendingButton.vue` (each + `.test.ts`), `src/styles/pending.css`.

Changed:
- `src/router/index.ts` — first `beforeEach`/`afterEach`; **a separate `onError` registered before the existing handler** (per §2).
- `src/App.vue` — mount `<ActivityBar>`; retire the global `.spinner`/`.loading-state` duplication; global reduced-motion rule.
- `src/styles/scales.css`, `src/main.ts`.
- Buttons → `<PendingButton>`: `DynamicForm.vue:1819`, `SettingsView.vue:1302,1372,1452`, `SearchView.vue:391`, `AnalyzeView.vue:142`. (**ConfirmModal excluded** per §3b — so 6 sites, not the ticket's "~12"; the ticket's figure is corrected here.)
- Block spinners → gate: `EntityList.vue:839`, `EntityDetail.vue:704`, `DynamicForm.vue:1653`, `KanbanView.vue:547`, `DashboardView.vue:189`, `SettingsView.vue:716`, `SearchView.vue:412`, `AnalyzeView.vue:146`, `HelpModal.vue:94`, `SidePanel.vue:91`; bare-text sites `ConflictsView.vue:135,205`, `HistoryView.vue:240`, `RelationHistoryView.vue:220`, `RelationCards.vue:531`, `RelationPicker.vue:411`.
- **`DocumentView.vue:168` / `DocumentsPanel.vue:189` — per RR-TCZWUI (minor): already correct. PRESERVE the `&& !docContent` guard.** Adding the delay gate is optional polish; removing that condition would reintroduce blanking on refresh, the exact regression this ticket exists to prevent.
- **`DocumentView.vue:162` / `DocumentsPanel.vue:183` `.spinner-sm`** — icon-only-control case; swap the icon in place in the same box. Separate line item from the block spinners.
- Delete the 10 duplicated `@keyframes spin` blocks and locally-redefined `.spinner`/`.loading-state`.

Done in planning: `EntityList.test.ts` (+2 tests), `EntityList.vue` (comment
correction).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `label` / `pendingLabel` — operator/developer-authored, not user data. Text interpolation, never `v-html`; no XSS surface. Where a label comes from `data-entry.yaml` it is config, which per the root CLAUDE.md is explicitly not secret.
- `delay` / `minDuration` — numeric props with defaults; non-finite or negative values coerce to the default rather than arming a broken timer.
- No new network calls, file access, auth or crypto. Presentation-layer only.

**Security-Sensitive Operations:** One real concern, raised by RR-R5VL59:
`aria-disabled` does not prevent activation, so the handler must be suppressed
in JS **for pointer and keyboard alike**. On a destructive confirm a second
activation is a second DELETE, so this is a correctness/idempotence issue, not
merely cosmetic. Mitigated by scoping `aria-disabled` to primary actions only
(§3a) and by AC9 testing both activation paths. The server remains the
authority.

Error text is unchanged: existing sites already route through `getErrorMessage`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1-AC9 each name their mechanism. Unit tests use
`vi.useFakeTimers()`; the gate is pure logic. Component tests mount
`PendingButton`/`ActivityBar` with fake timers. Integration: `EntityList`
(done), an `App.vue`-level test driving the **real router** through overlapping
and cancelled navigations, and an SSE test asserting a background refetch drives
nothing.

**Edge Cases:**
- Source flips true→false→true inside one delay window (double-click): no stacked timers, no double-show (topbar.js's `if (delayTimerId) return` guard).
- Unmount while in `DELAY`/`DISPLAY`/`EXPIRE`: timers cleared, no post-unmount ref writes (cf. `RR-YWWAL`, the same leak class in `useConfirm`).
- Operation **fails** fast (rejects at 50ms): the minimum must not hold a stale "Saving…" over an error toast — failure clears immediately, bypassing `EXPIRE`.
- **Cancelled / aborted / duplicated navigation** (the RR-B7U3I8 case): bar must clear. Explicit leak test.
- Redirect chains (`beforeEach` firing twice before one `afterEach`): identity-tracking handles this; assert it.
- `minDuration: 0` / `delay: 0`: degrade to instantaneous, no timer armed.
- Cold load vs. param change on lists (both pinned).
- Reduced motion: motion suppressed, state still conveyed.

**Negative Tests:**
- Removing `placeholderData` must fail the pagination test — **already verified by mutation test.**
- Gating the list template on `isLoading`/`asyncStatus` instead of `isPending` must fail the same test.
- A `PendingButton` without `pendingLabel` must fail typecheck.
- A pending primary button whose handler still fires on click **or on Enter/Space** must fail AC9.
- Forcing a cancelled navigation must not leave `.activity-bar` displayed.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
1. **Breadth of migration (~23 sites).** Mitigation: land primitives + tokens first with tests, then migrate in independently-revertable batches (buttons → block spinners → bare-text).
2. **Two async systems.** Mitigation: the gate takes `Ref<boolean> | (() => boolean)`, so Colada's `isPending` and legacy `ref(false)` both feed it unchanged.
3. **Router integration is the highest-risk change** (raised from "low" by RR-B7U3I8). It adds the app's first navigation guards, to a router whose error handler is already carrying three documented Firefox/Vite race workarounds. Mitigation: identity-tracking (leak-proof by construction), a separate `onError` registration that cannot be skipped by the existing early-returns, and an explicit cancelled-navigation leak test. Do not refactor the existing `onError` body.
4. **Timings are unvalidated by feel.** Mitigation: single-sourced tokens; re-evaluate after living with it.
5. **jsdom cannot verify the width contract.** Mitigation: structural unit assertions + one e2e measuring `getBoundingClientRect().width`.
6. **Deleting 10 duplicated `@keyframes spin` blocks.** Mitigation: full suite + e2e; definitions are byte-identical apart from `RelationCards`'s 0.6s.
7. **Equal-width grouping could distort an asymmetric pair.** Mitigation: genuine pairs only; documented limits.

**Effort:** `l` — confirmed.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `frontend/CLAUDE.md` — **required.** New "Pending and loading indicators" section: the three classes, which primitive per class, the governing rule, the ban on ad-hoc spinners/skeletons, **the `ConfirmModal` exception (§3b)**, **the `aria-disabled`-primary-only rule (§3a)**, and **the debounce-is-the-ambient-delay note (§6)**. Without it the next contributor hand-rolls indicator #19.
- [x] `docs/data-entry.md` — brief user-visible note about the navigation progress bar and inline pending labels.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel surface)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface)
- [x] ~~Root `CLAUDE.md`~~ (N/A: frontend-local convention, documented in `frontend/CLAUDE.md`)
- [x] ~~`README.md`~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**
- `RR-B7U3I8` (**critical**) — ActivityBar counter leaks on cancelled navigation. Addressed: Approach §2 replaces counting with identity-tracking; AC5 restated; risk 3 raised; leak test added.
- `RR-VFI1W0` (**significant**) — `PendingButton` API does not fit `ConfirmModal`. Addressed: §3b removes it from scope as a sanctioned exception; call-site count corrected from ~12 to 6.
- `RR-R5VL59` (**significant**) — `aria-disabled` scope unspecified. Addressed: §3a limits it to primary actions; Cancel keeps native `disabled`; AC9 extended to keyboard activation.
- `RR-ZT9DXG` (**significant**) — ambient timing ignores the 800ms debounce. Addressed: §6 documents the debounce as the ambient class's effective delay and corrects the ticket's rationale.
- `RR-TCZWUI` (**minor**) — migration list would regress `DocumentView`/`DocumentsPanel`. Addressed: both annotated "already correct, preserve the `!docContent` guard"; icon-only spinners split into their own item.
