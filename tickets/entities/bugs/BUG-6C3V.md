---
id: BUG-6C3V
type: bug
title: 'Firefox: cancelled-fetch errors logged during rapid navigation in the data-entry SPA'
description: 'When the user opens an entity-detail page in Firefox and then navigates to another page (via reload+back, or direct goto+click on another row) before the first detail page''s loadCommands() fetch settles, the in-flight commands fetch is cancelled by Firefox and EntityDetail.vue logs ''Failed to load commands: Error'' to the console. Chromium does not show this because it handles cancelled fetches differently. Found by frontend/stress fuzzer (mode=fuzz, browser=firefox) in 35 seconds across 4 examples + 6 shrinks. Two distinct minimal failing sequences from different seeds: (1) click-row index=0 -> reload -> back; (2) click-row index=0 -> goto all_tickets -> click-row index=8. Both reduce to ''mount entity-detail twice in a row before the first mount has finished its async setup''. The fix is likely to abort or ignore in-flight loadCommands requests when the EntityDetail component unmounts — the same pattern any Vue async setup needs for navigation safety.'
priority: medium
effort: m
why1: loadCommands() awaits a fetch that gets cancelled when the EntityDetail component unmounts mid-fetch.
why2: EntityDetail.vue does not abort in-flight requests on unmount; the catch block treats the cancelled-fetch error as a real failure and console.errors it.
why3: 'Empirical: the canonical Vue idiom (AbortController + onBeforeUnmount + axios.isCancel) does NOT catch this in axios 1.6.7. axios surfaces AbortController-driven aborts as AxiosError with code=''ECONNABORTED'' or ''ERR_CANCELED'', and axios.isCancel() only matches the legacy CanceledError from the deprecated CancelToken API. Furthermore, when the user navigates away in Firefox the underlying fetch is aborted by the browser (not by our AbortController), so localAbort.signal.aborted is also false. The reliable check is `err.code === ''ECONNABORTED'' || err.code === ''ERR_CANCELED''`.'
why4: 'The Vue/axios ecosystem documentation tells you to use AbortController + isCancel, but the API surface has drifted: AbortController is the canonical primitive yet isCancel only recognizes the legacy CancelToken type. There is no version-portable, framework-recommended way to recognize ''this fetch was cancelled by either us or the browser''. Each developer has to discover the ECONNABORTED quirk by hand.'
prevention: The canonical Vue 3 AbortController + onBeforeUnmount + axios.isCancel recipe doesn't work in axios 1.6.7 — isCancel only matches legacy CanceledError, not AbortController-driven aborts, and browser-driven cancellation bypasses our AbortController entirely. Every component in the SPA that calls an axios method from a lifecycle hook needs either the isCancelledFetch helper (which checks err.code === 'ECONNABORTED' || err.code === 'ERR_CANCELED' in addition to the signal.aborted path) or the usePageData composable. Future async loads should use the composable. The fast-check fuzzer in frontend/stress/fuzzRunner.ts is the long-term regression guard — it finds these bugs in seconds via random action sequences and shrinks them to minimal counter-examples.
status: ready
---

## Symptom

In Firefox, the data-entry SPA logs cancelled-fetch errors to the browser
console during rapid navigation (reload, back, click another row, etc.).
Chromium does not show these — it silently drops cancelled fetches. The errors
include `Failed to load commands: Error`, `Failed to load ${entityType}`,
`Failed to load entities`, `Failed to fetch git status`, `Failed to load
sidebar`, and a family of bare `undefined` errors coming from vue-router's
internal error path.

## Discovered by

The fast-check fuzzer in `frontend/stress/fuzzRunner.ts`. Initial reproduction
in 35 seconds, 4 examples, 6 shrinks. Iterative fuzz→fix→fuzz cycle peeled five
distinct layers (see "Layers peeled" below) before 1000+ Firefox sequences run
clean.

## Layers peeled

| Layer | Error surface | Root cause | Fix |
|---|---|---|---|
| 1 | `Failed to load commands: Error` | `EntityDetail.vue:loadCommands` awaits `getCommands` with no abort/cancellation handling | AbortController + onBeforeUnmount in `EntityDetail.vue`, `isCancelledFetch(err)` check |
| 2 | `Failed to load ${type}` / `Failed to load entities` | Same race shape in `EntityDetail.vue:loadEntity` (calls `entitiesStore.fetchEntity`) and `EntityList.vue:loadEntities` (calls `entitiesStore.fetchList`) | `isCancelledFetch(err)` check in each catch |
| 3 | Bare `undefined` at `vue-router.mjs` | vue-router's internal `triggerError` calls `console.error(error)` when no error listener is registered, and Vite's dynamic `import()` chunk loader sometimes rejects with `undefined` when the fetch is interrupted by navigation | Registered `router.onError` handler in `router/index.ts` that filters `NavigationFailure` subtypes, `isCancelledFetch` errors, `undefined`/`null`, and chunk-loader rejection messages |
| 4 | `Failed to fetch git status: Error` | `stores/git.ts:fetchStatus` — Pinia store action with same race | `isCancelledFetch(err)` check |
| 5 | `Failed to load sidebar: Error` (proactive) | `Sidebar.vue:loadSidebar` — same race, fixed proactively from the audit scope | `isCancelledFetch(err)` check |

## The axios quirk

The canonical Vue ecosystem recipe is `AbortController + onBeforeUnmount +
axios.isCancel(err)`. **This does not work in axios 1.6.7** (and appears to be
the same in later versions). Two distinct cases fail:

1. **`axios.isCancel(err)` only matches `CanceledError`**, which is what
the legacy `CancelToken` API throws. Modern `AbortController`-driven aborts
surface as `AxiosError` with `code === 'ECONNABORTED'` or `code ===
'ERR_CANCELED'`. `isCancel` returns false for them.
2. **Browser-driven cancellation bypasses our `AbortController`
entirely.** When Firefox aborts an in-flight fetch because the user navigated
away, our `controller.signal.aborted` stays `false` but axios still surfaces the
failure as `ECONNABORTED`. Checking the signal we passed in is not sufficient.

The reliable check is `err.code === 'ECONNABORTED' || err.code ===
'ERR_CANCELED'`, or the `isCancelledFetch` helper that wraps both this and the
legacy `AbortError`/`CanceledError` name check.

## The vue-router chunk-loader issue

Route components are lazy-loaded via `() => import('@/views/X.vue')`. During
rapid navigation in Firefox, the dynamic import can reject with `undefined` (not
a real Error) when the fetch for the chunk is cancelled mid-flight. vue-router's
`loadRouteLocation` propagates this to `triggerError`, which calls
`console.error(undefined)` if no `router.onError` handler is registered. We
can't fix Vite's chunk loader from the application level; the right fix is a
router error handler that silences this specific class.

## Final fix

**`frontend/src/composables/usePageData.ts`** — new composable. Exports
`isCancelledFetch(err)` helper that recognises both axios error codes
(`ECONNABORTED`, `ERR_CANCELED`) and DOMException names (`AbortError`,
`CanceledError`). Also exports a `usePageData()` composable for future async
loads to consolidate the pattern; not used by the existing narrow fixes but
available for new code.

**`frontend/src/router/index.ts`** — registered `router.onError` with documented
filter list: silences NavigationFailures (cancelled/aborted/duplicated),
`isCancelledFetch` errors, `undefined`/`null`, and chunk-loader rejection
patterns. Anything else is logged with navigation context.

**`frontend/src/main.ts`** — global `app.config.errorHandler` and
`window.addEventListener('unhandledrejection')` hooks. Production-grade error
capture with component name attribution. Stays in even after bug is fixed.

**`frontend/src/api/client.ts`** — `ApiClient.get(url, params, signal?)` now
accepts an optional abort signal and forwards it to axios.

**`frontend/src/api/commands.ts`** — `getCommands(params, signal?)` forwards the
signal.

**`frontend/src/components/entity/EntityDetail.vue`** — `loadCommands` uses
per-call `AbortController` (cancels previous on new call), aborts on
`onBeforeUnmount`. Both `loadCommands` and `loadEntity` catches use
`isCancelledFetch(err)` to silence cancellations.

**`frontend/src/components/lists/EntityList.vue`** — `loadEntities` catch uses
`isCancelledFetch(err)`.

**`frontend/src/stores/git.ts`** — `fetchStatus` catch uses
`isCancelledFetch(err)`.

**`frontend/src/components/common/Sidebar.vue`** — `loadSidebar` catch uses
`isCancelledFetch(err)`.

## Verification

| Run | Browser | Sequences | Max actions | Result |
|---|---|---|---|---|
| Layer 1 replay | Firefox | 20 | 3 (fixed) | 0/20 |
| Layer 1 replay | Chromium | 10 | 3 (fixed) | 0/10 |
| Layer 5 fuzz | Firefox | 200 | 15 | 0/200 |
| Layer 6 fuzz | Firefox | 500 | 20 | 0/500 |
| Layer 7 fuzz | Firefox | 1000 | 20 | 0/1000 |
| Layer 7 fuzz | Chromium | 200 | 15 | 0/200 |

Total Firefox sequences explored across all post-fix runs: **over 2,200, zero
failures**.

## What's NOT fixed

1. The user's original "long long time" Firefox hang is not directly
reproduced by the fuzzer. The errors we fixed are in the same family
(Firefox-specific, navigation-related) but not provably the same root cause. A
tighter fuzzer oracle is filed as follow-up work.
2. Other components with the same pattern (`HelpModal.vue`,
`SidePanel.vue`, `RelationPicker.vue`, `CustomView.vue`, `DynamicForm.vue`) are
not yet fixed. They will surface from the fuzzer as new bugs if the vocabulary
is extended to reach those code paths.
3. The `usePageData` composable is written but not adopted — it's a
follow-up refactor to consolidate all the `isCancelledFetch` catches into a
single composable.

## 5-Whys

- **why1**: The SPA's async load functions (`loadCommands`, `loadEntity`, etc.) log cancellation errors to the console when the user navigates away before the fetch settles.
- **why2**: Each component wraps its fetch in a try/catch that treats the AxiosError as a real failure without checking whether it was a cancellation.
- **why3**: The canonical Vue 3 + axios idiom for cancellation handling does not work in axios 1.6.7 — `axios.isCancel` only catches the legacy CancelToken path, not AbortController, and the browser-cancelled case bypasses AbortController entirely.
- **why4**: The Vue / axios ecosystem documentation tells developers to use AbortController + isCancel, but the axios API surface has drifted: the modern primitive (AbortController) is not recognised by the modern helper (isCancel). Each developer has to discover the ECONNABORTED quirk by hand, usually after seeing the bug in Firefox.
- **why5**: There is no project-level convention for handling async setup unmount races in this SPA, so every new component re-invents the catch block. The systemic fix is a `usePageData` composable that bakes in the correct cancellation check, plus the fuzzer as a regression guard that catches new instances of the pattern within seconds of being introduced.

## Prevention

- **`isCancelledFetch` helper in `src/composables/usePageData.ts`** is now the canonical cancellation check. All new async loads in the SPA should either use the `usePageData` composable or, at minimum, call `isCancelledFetch(err)` in their catch block before logging.
- **`router.onError` handler in `src/router/index.ts`** catches the vue-router class of errors once-globally.
- **Global `errorHandler` in `src/main.ts`** provides component-attributed error logging for anything the per-component catches miss.
- **The fast-check fuzzer** (`npm run stress -- --mode=fuzz --browser=firefox`) is the standing regression guard. Run it on any PR that touches the SPA's routing or component lifecycle.
