---
id: RR-FB2A
type: review-response
title: 'Round 2 NEW-1: response-merge race on :key remount'
finding: |
  The plan's `:key="entry.type/entry.id"` on SectionEditForm correctly targets PATCHes during unmount (props still resolve to entity A). BUT the PATCH RESPONSE arrives later and runs `mergeServerResponse → applyServerProperty → onPropertyApplied (handlePropertyApplied)` in the OLD closure. `handlePropertyApplied` reads `viewData.value`, which by then points to B. Result: entity A's response leaks into entity B's view.
severity: critical
status: addressed
resolution: |
  SectionEditForm captures its entity identity at construction (`const ownEntity = { type: props.entityType, id: props.entityId }`) and forwards it through `onPropertyApplied`. The callback signature becomes `(prop: string, value: unknown, ownerEntity: { type: string; id: string }) => void`. EntityDetail's `handlePropertyApplied` rejects writes that don't match the current `viewData.entry`:

  ```ts
  function handlePropertyApplied(prop, value, owner) {
    const view = viewData.value
    if (!view?.entry) return
    if (view.entry.type !== owner.type || view.entry.id !== owner.id) return // stale response from previous entity
    viewData.value = { ...view, entry: { ...view.entry, properties: { ...view.entry.properties, [prop]: value } } }
  }
  ```

  This makes the unmounting-instance's response a no-op on the new entity's view. The new (re-mounted) SectionEditForm against entity B has a fresh autosave instance with B's baseline; A's response is genuinely lost (acceptable: the user already left A and B's view is canonical).

  PLAN AC 5 amended; signature of onPropertyApplied widened in AC 4.
---
