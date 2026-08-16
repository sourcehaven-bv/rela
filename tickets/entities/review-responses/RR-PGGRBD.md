---
id: RR-PGGRBD
type: review-response
title: Do not pass `render` as isFieldWritable's second `fieldReadonly` parameter
finding: '`isFieldWritable(verdict, fieldReadonly?)` (frontend/src/utils/affordances.ts:12-18) already has a static-readonly second parameter that SectionEditForm does not use. It is an obvious-looking home for `render`, and the plan never explicitly forbids it. Doing so would be wrong: `isFieldWritable` is called both at widgetRows (SectionEditForm.vue:142) and in the verdict-flip watcher (:172-173); passing `render` symmetrically at both sites reintroduces exactly the spurious ''Permission changed'' toast that risk 3 exists to prevent. The doc comment at affordances.ts:9-11 also notes view-side hosts have no static-readonly concept.'
severity: minor
resolution: 'Plan updated with an explicit prohibition: `render` stays a separate conjunct at the widgetRows call site only (`writable: field.render === ''input'' && isFieldWritable(field.verdict)`); the `fieldReadonly` parameter is left unused by view-side hosts, and the watcher continues to read `verdict` alone. Also confirmed `isFieldWritable(undefined) === true`, so `render: input` with no ACL verdict is editable — the intended behaviour.'
status: addressed
---

Verified:

```ts
// frontend/src/utils/affordances.ts:12-18
export function isFieldWritable(verdict, fieldReadonly?): boolean {
  if (fieldReadonly) return false
  return verdict?.writable !== false
}
```

Two call sites in `SectionEditForm.vue` — `:142` (widgetRows) and `:172-173`
(watcher). The conjunct must be applied at the first only. This is the mechanism
behind risk 3, stated explicitly so an implementer doesn't "tidy" the conjunct
into the parameter.
