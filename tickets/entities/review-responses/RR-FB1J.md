---
id: RR-FB1J
type: review-response
title: 'S6: ViewSectionField.property is optional — must filter before iterating'
finding: |
  `ViewSectionField.property?: string` is optional on the wire. The plan dereferences it unconditionally, leading to `_fields[undefined]?.writable !== false` evaluating true (rendering SectionEditForm with an unkeyable field) and ultimately `scheduleFieldSave(undefined, value)`.
severity: significant
status: addressed
resolution: |
  EntityDetail-side helper filters out non-property fields before building SectionEditForm's `fields` prop: `const editableFields = section.fields?.filter(f => f.property) ?? []`. If after filtering no writable fields remain, EntityDetail falls back to PropertyDisplay for the entire section (consistent with RR-FB1K mixed-render decision). SectionEditForm's `fields` prop is typed `property: string` (required) so downstream callers can't reintroduce the bug.
---
