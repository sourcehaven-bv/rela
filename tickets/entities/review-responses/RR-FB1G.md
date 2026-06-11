---
id: RR-FB1G
type: review-response
title: 'Reactivity: direct mutation of entry.properties[p] may not trigger sibling re-renders'
finding: |
  PLAN's `onPropertyApplied: (p, v) => { entry.properties[p] = v }` mutates a reactive object directly. Vue 3's reactivity tracks property additions via Proxy, so this should work for existing keys, but it's brittle: any sibling section that destructures `properties` into a precomputed array won't re-render. Better: spread-clone via `viewData.value = { ...viewData.value, entry: { ...entry, properties: { ...properties, [p]: v } } }`. This is what EntityDetail's existing checkbox-toggle path does (after IHC7A).
severity: significant
status: addressed
resolution: |
  PLAN uses the spread-clone shape consistent with EntityDetail's existing content-channel `applyServerContent` (lines 192-205 of current EntityDetail.vue). The `onPropertyApplied` impl in EntityDetail becomes:

  ```ts
  onPropertyApplied: (prop, value) => {
    const view = viewData.value
    if (!view?.entry) return
    viewData.value = {
      ...view,
      entry: { ...view.entry, properties: { ...view.entry.properties, [prop]: value } },
    }
  }
  ```

  Plus the same `pinEntityForFlush`-style guard (or simpler: with RR-FB1D's :key remount, the stale-target issue is naturally avoided).
---
