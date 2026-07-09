---
id: RR-6R4P6A
type: review-response
title: Stale WidgetProps.entityType docstring
finding: types.ts WidgetProps.entityType comment says 'present only on the file-property edit path'; SelectWidget/MultiSelectWidget now consume it as the enum-label disambiguator and FieldRenderer forwards it broadly. Update the comment.
severity: nit
resolution: Updated WidgetProps.entityType docstring to note it is also the enum-label disambiguator consumed by the select widgets and forwarded broadly by FieldRenderer; split out the entityId comment.
status: addressed
---
