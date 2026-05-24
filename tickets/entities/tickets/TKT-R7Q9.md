---
id: TKT-R7Q9
type: ticket
title: Migrate checkbox toggle to PATCH-based reactive flow; retire /api/toggle-checkbox
kind: enhancement
priority: medium
effort: s
status: done
---

Replace the legacy `/api/toggle-checkbox` endpoint with the unified PATCH path
(`PATCH /api/v1/{plural}/{id}`), and shift the frontend from refetch-everything
to reactive entity-state mutation.

## Motivation

After BUG-N6WW restored checkbox toggling, every click triggers a full
`loadView()` refetch that flips `loading.value = true` for the whole component.
The `v-if="loading"` template branch tears down and rebuilds the entire
entity-detail tree (header, sections, properties, content body, documents
panel), producing a visible flicker on every toggle.

The root cause isn't the bug fix — it's the imperative refetch pattern the
toggle flow was built on. Today's `/api/toggle-checkbox` is a legacy endpoint
predating the Vue SPA (originally consumed by htmx). Its response is server-side
goldmark-rendered HTML the SPA throws away. The SPA then refetches the
ViewResponse to get fresh source markdown to feed into its own marked renderer.
Coarse, lossy, flickery.

## Approach

1. **Port `toggleCheckbox` source-toggling logic to TS** (sibling of `frontend/src/utils/markdown.ts`). Mirrors `internal/dataentry/helpers.go:397`. Table-test against the same cases as `TestToggleCheckbox`.
2. **Switch the toggle flow** in `EntityDetail.vue`: `contentClick` calls the new TS toggler against `viewData.entry.content`, fires `PATCH /api/v1/{plural}/{id}` with `{ content: newSource }`, and assigns the returned entity back to `viewData.entry`. Vue's reactivity re-renders only the content section and the (n/m) stats counter. No `loadView()`, no `loading.value` flip, no flicker.
3. **Retire `/api/toggle-checkbox`**: remove handler, route registration. Keep the `toggleCheckbox` helper in `helpers.go` with its tests if it has other callers (it doesn't — the only caller is the retiring handler), otherwise delete it.
4. **Update `data-entry-ui` concept doc** to drop `toggle-checkbox` from the legacy `/api/*` endpoint list.

## Side benefits

- One write surface for entity mutations (PATCH everywhere); fewer endpoints to maintain, version, secure.
- Consistent error handling — PATCH returns the structured 200-with-warnings / 422 / 400 shape from DEC-HWZHA. Today's `/api/toggle-checkbox` returns plain-text errors with HTTP codes.
- ETag-based optimistic locking comes for free (caller can opt in via If-Match).
- Removes the multipart-form-data quirk BUG-N6WW worked around (PATCH is JSON).

## Out of scope

- Optimistic UI (flip the visual checkbox before the server confirms). Could layer on later but not needed here — the PATCH round-trip is ~50ms in practice; the reactive re-render is instant.
- Server-side index bounds check (RR-VD9L) — index-out-of-range now happens client-side before the PATCH is even sent.
