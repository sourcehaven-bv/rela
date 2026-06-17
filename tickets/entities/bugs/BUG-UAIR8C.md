---
id: BUG-UAIR8C
type: bug
title: Vue unmount crash navigating away from entity-detail pages with a populated display:list relation section
description: |-
    Navigating away (any in-SPA router transition) from an /entity/:type/:id detail page that renders a configured `display: list` relation section with at least one row throws an uncaught Vue-internal error during the route-driven unmount:

    - dev build: `TypeError: Cannot read properties of null (reading 'type')` in `unmountComponent`
    - prod build: `TypeError: Cannot destructure property 'bum' of 'e' as it is null` (Vue runtime error #15)

    Reported as GitHub issue #997. Fully deterministic. Surfaces ONLY through Vue's app-level `config.errorHandler` — `pageerror` / `console.error` listeners never see it, which is why it slipped manual and automated testing.

    **Root cause:** the cards/list inline-edit rows (TKT-IHC7C) render the per-row `AutoSaveIndicator` via `<Teleport :to="#list-indicator-${type}-${id}">`, but the target `<span>` lives INSIDE the same render subtree (inside the row's `<a class="list-link">` / card `<header>`) as the Teleport itself. Vue resolves the `to` selector via querySelector at mount; because the target is created in the same render flush, the teleport silently fails to deliver — confirmed empirically: at steady state the target span is empty and the AutoSaveIndicator is absent from the entire EntityDetail subtree. The orphaned teleported child has a null component instance, and Vue's `unmountComponent` destructures that null on route-driven teardown → crash.

    The `cards` branch is structurally identical (`card-indicator-*`, span inside `<header>`) and carries the SAME latent bug; it only escaped because no in-tree view configured a `display: cards` relation section that actually mounted an inline-edit row, so its teleport was never exercised.

    **Fix:** drop the `<Teleport>` in both the list and cards branches and render the `AutoSaveIndicator` inline inside the row's `SectionEditForm` (slot scope preserved), pinned top-right via CSS so placement matches the original TKT-IHC7C design. With no cross-subtree target resolution there is no orphaned vnode to corrupt unmount. (`frontend/src/components/entity/EntityDetail.vue` only.)
priority: high
effort: s
why1: Vue's `unmountComponent` read/destructured a null component instance (`instance.type` / `const { bum } = instance`) while tearing down the EntityDetail subtree on route navigation.
why2: 'The null-instance vnode was an orphaned `<Teleport>` child: the teleported `AutoSaveIndicator` was never delivered into its target, so on unmount Vue walked a teleport whose mounted child component was null.'
why3: The `<Teleport :to="#list-indicator-...">` target `<span>` was rendered inside the SAME component render subtree (inside the row's `<a>` wrapper) as the Teleport. Vue resolves `to` via querySelector at mount; the target isn't reliably in the DOM at that flush, so the teleport silently no-ops and leaves the child orphaned.
why4: TKT-IHC7C chose a Teleport-into-row-chrome design to position one indicator per row (RR-FC1D / RR-FC2A reopened the placement decision twice) without recognizing that teleporting into the same unmounting subtree is a fragile Vue pattern. Review focused on WHERE the indicator sits, not on teleport target lifetime/teardown safety.
why5: The feature shipped with only a pure-logic unit test (`sectionEditFields.test.ts`, covering the decision of whether to mount a row form) — no EntityDetail.vue component-level mount/unmount test and no e2e navigate-away test. The crash is invisible to console/pageerror hooks, so no layer of the test suite exercised the unmount path of a populated list/cards section.
prevention: |-
    1. Suite-wide e2e guard (the real prevention): the `appPage` fixture in `e2e/tests/fixtures.ts` now captures the SPA's global `app.config.errorHandler` output (logged as `[vue-error] ...` console lines by `frontend/src/main.ts`) and fails the test in afterEach. Because the handler catches framework-swallowed lifecycle/render/unmount errors that never reach `pageerror`/`console.error`-as-exception, EVERY navigation in EVERY spec is now an unmount-error probe — not just the dedicated regression test. Verified by reverting the fix: the guard fails with the exact `Cannot destructure property 'bum' ...` message; restored, all 204 e2e tests pass.
    2. Dedicated regression `e2e/tests/entity-detail-list-unmount.spec.ts`: the inline fixture configures a `task` view (kept off `feature` so it doesn't perturb other specs' default-feature assertions) with a populated `display: list` section (TASK-001 implements FEAT-001); the test mounts that shape (guarded by `expectListSectionRowMounted`) and navigates away via an in-SPA router link, leaving the assertion to the fixture guard.
    3. The SPA already wires a global `app.config.errorHandler` → console (`frontend/src/main.ts`); this is what surfaces the class of error and what the e2e guard keys on.
    4. Rule of thumb recorded: never `<Teleport :to>` into an element rendered inside the same unmounting subtree; render inline or teleport only to a stable ancestor that outlives the source.
status: done
---
