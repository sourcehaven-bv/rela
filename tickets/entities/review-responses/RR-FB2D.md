---
id: RR-FB2D
type: review-response
title: 'Round 2 NEW-4..NEW-10: minor concerns'
finding: |
  - NEW-4: `watch(() => props.fields)` fires on every parent render because buildSectionEditFields returns new array each time.
  - NEW-5: applyServerProperty(prop, undefined) should delete formData[prop] (not set undefined), mirroring DynamicForm L923-929.
  - NEW-6: 401 should refetch like 403.
  - NEW-7: onError info on content/relations channels untested.
  - NEW-8: pendingRefetch race window documented.
  - NEW-9: AutoSaveOptions required-fields no-op list documented for SectionEditForm.
  - NEW-10: formData and initialServerSnapshot must spread independently.
severity: minor
status: addressed
resolution: |
  PLAN amended:
  - NEW-4: EntityDetail wraps `buildSectionEditFields(section, entry)` in `computed(() => ...)` keyed off section and entry so the array identity stabilises across renders. SectionEditForm's `watch` only fires when verdicts actually change.
  - NEW-5: SectionEditForm's applyServerProperty deletes the key when value is undefined, matching DynamicForm. handlePropertyApplied likewise: `if (value === undefined) delete next.properties[prop]; else next.properties[prop] = value`.
  - NEW-6: handleSectionEditError refetches on `info.status === 401 || info.status === 403`.
  - NEW-7: useAutoSave.test.ts gains content + relations channel onError-info smoke tests.
  - NEW-8: documented as an accepted race; no fix needed.
  - NEW-9: PLAN AC 4 enumerates the no-op closures and refs SectionEditForm passes for unused channels.
  - NEW-10: PLAN AC 4 explicitly spreads initialValues twice (once for formData, once for initialServerSnapshot.properties).
---
