---
id: BUG-ZE4354
type: bug
title: Cancel on a directly-opened create form walks out of the SPA (router.back with no history)
description: 'On a create form opened directly (pasted URL, bookmark, new tab), Cancel appears to do nothing. handleCancel calls router.back(), a browser-history operation with no route target; with no prior SPA entry it walks out of the app entirely (measured: lands on about:blank). Works when the form is reached via an in-app link. Pre-existing on develop, not a TKT-OMUD56 regression. Fix: prefer the already-resolved returnTo (which the submit path honours but Cancel ignores), fall back to history only when an in-app entry exists, else push the type''s list view.'
priority: medium
status: backlog
---

## Symptom

On a create form opened **directly** (pasted URL, bookmark, new tab), the
**Cancel** button appears to do nothing.

## What actually happens

It is not a no-op. `handleCancel` calls `router.back()`
(`frontend/src/components/forms/DynamicForm.vue:1019`), which is a *browser
history* operation, not a route target. With no prior SPA entry in the tab's
history, back walks **out of the app** — to whatever preceded it. In a normal
tab that is the new-tab page, which reads to the user as "nothing happened".

Measured with Playwright against the running app:

| Case | Before | After Cancel |
| --- | --- | --- |
| Opened directly in a fresh tab | `/form/create_ticket` | `about:blank` |
| Arrived via an in-app link | `/form/create_ticket` | `/list/all_tickets` |

So the button works whenever the form is reached the normal way (sidebar, a
list's "+ New", a detail page) and only misbehaves on direct entry.

## Reproduction

1. Open a new browser tab.
2. Paste `http://<host>/form/create_ticket` and load it.
3. Click **Cancel**.

Expected: return somewhere sensible in the app. Actual: leaves the SPA.

## Scope

Pre-existing, not a regression: `handleCancel` was `router.back()` on `develop`
before TKT-OMUD56, which only added an `embedded` early-return above it (the
nested-modal path emits `inline-cancelled` and never reaches `router.back()`).
Found while manually testing that ticket's demo.

Applies to the edit form too — where the button is labelled **Back** — since
both go through the same `handleCancel`.

## Suggested fix

Prefer an explicit target, fall back to history, then to a sensible default. The
pieces already exist:

- `returnTo` is already resolved in this component from a validated
`?return_to=` (`applyReturnToFromQuery`, using `readReturnTo`'s open-redirect
guard) and is already honoured by the **submit** path — Cancel simply does not
consult it. That asymmetry is the core of the bug.
- `useBackTarget` (`frontend/src/composables/useBackTarget.ts:44`) already
encodes the resolution order `return_to` → `?from=` → null, and is the
established precedent for "where does back go" in this SPA.
- `findListIdForEntityType` (`stores/schema.ts`) resolves a type's list view
for a last-resort default.

Sketch:

```ts
function handleCancel() {
  if (props.embedded) { emit('inline-cancelled'); return }
  if (returnTo.value) { router.push(returnTo.value); return }
  // Only go back when there is an in-app entry to go back to.
  if (hasAppHistory()) { router.back(); return }
  const listId = schemaStore.findListIdForEntityType(formConfig.value.entity)
  router.push(listId ? `/list/${listId}` : '/')
}
```

The history check is the load-bearing part — `window.history.length` is
unreliable (it counts pre-SPA entries), so prefer tracking whether this SPA
instance has navigated at all (e.g. a router `afterEach` counter, or
`router.options.history.state.back != null`).

Reusing `useBackTarget` rather than re-deriving the order would keep the two
surfaces consistent.

## Test plan

- Unit: Cancel with `returnTo` set pushes it; without history pushes the list
fallback; with history calls `back()`.
- E2E: open a create form directly, click Cancel, assert the app is still
mounted and on a sensible route (currently would land off-app).
- Regression: arriving via a list "+ New" still returns to that list.
