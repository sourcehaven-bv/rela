---
id: RR-FB1D
type: review-response
title: 'C4: EntityDetail does not remount on route change — SectionEditForm needs explicit key or watch'
finding: |
  Router config `/entity/:type/:id` does NOT key the route component. EntityView.vue does NOT key the inner EntityDetail. Navigating `entity/ticket/A` → `entity/ticket/B` reuses the same EntityDetail instance; the inner `<SectionEditForm>` would also be reused, with stale baseline and entity-id pinning to the wrong target on the next debounce fire. The plan's "props still point at previous entity during unmount" claim is false — there's no unmount.
severity: critical
status: addressed
resolution: |
  Adopt `:key="\`${entry.type}/${entry.id}\`"` on `<SectionEditForm>` in EntityDetail's template. Each entity-id change forces a SectionEditForm remount, which triggers its `onBeforeUnmount → commitImmediately()` against the previous entity (props are still pointing at the previous entity until the remount completes), then mounts fresh against the new entity with a fresh `initialServerSnapshot`. Side-steps the pinEntityForFlush dance entirely. Documented in the implementation notes.

  Also resolves S8 (initialServerSnapshot re-seed on refetch — key forces remount on identity change) and S10/S12 (no pinEntityForFlush needed because remount runs the lifecycle).
---
