---
id: RR-Q99JT6
type: review-response
title: 'Pre-linked relation with no picker field aborted the entire create: pickerTypes had no entry for the peer'
finding: 'Found during implementation. pickerTypes is populated only by RelationPicker via updateRelationTypes, so a relation pre-linked through link_relation/link_peer that has no corresponding picker field on the form had no type entry for its peer id. reshapeLegacyToModern (relationsPatch.ts:126-142) returns null for any id it cannot type, and DynamicForm treats null as a pathological form: it shows "Some related entities have unknown types. Save aborted" and returns without creating anything. So a create button for a relation the form does not also render as a field could not save at all. This is pre-existing and affects the shipped page flow, not only the new modal flow — it was latent because the side-panel Add button never pre-linked in the first place (the param mismatch), so the combination had no live caller.'
severity: significant
resolution: 'The link prefill in initializeDefaults now registers the peer''s type in pickerTypes alongside its id, resolved via getTypeFromId — the same id-prefix source the post-create reverse link already uses. Pinned by DynamicForm.embedded.test.ts ''links via the prop'', which asserts the modern resource-identifier payload {type, id} and was mutation-verified: removing the registration makes create never fire.'
status: addressed
---

## Finding

Found while implementing the modal flow, but it is **pre-existing** and affects
the shipped page flow too.

`pickerTypes` is populated only by `RelationPicker` (via `updateRelationTypes`).
A relation pre-linked through `link_relation` / `link_peer` that has **no
corresponding picker field on the form** therefore had no type entry for its
peer.

`reshapeLegacyToModern` (`relationsPatch.ts:126-142`) returns `null` for any id
it cannot type, and `DynamicForm` treats `null` as a pathological form:

```
Some related entities have unknown types. Save aborted; reload the form and try again.
```

…and returns without creating anything. So **a create button for a relation the
form does not also render as a field could not save at all** — not a wrong link,
a refused create.

It was latent because the only caller was the side-panel Add button, and that
never pre-linked in the first place (the `_relation` param mismatch, AC8). The
two bugs masked each other: fixing the param names alone would have converted a
button that silently failed to link into a button that loudly failed to save.

## Resolution

The link prefill in `initializeDefaults` now registers the peer's type in
`pickerTypes` alongside its id, resolved with `getTypeFromId` — the same
id-prefix source the post-create reverse link already uses.

Pinned by `DynamicForm.embedded.test.ts` "links via the prop", which asserts the
modern resource-identifier payload (`{type: 'feature', id: 'FEAT-1'}`) rather
than a bare id list, so the type half cannot regress unnoticed.
Mutation-verified: removing the registration makes `create` never fire.
