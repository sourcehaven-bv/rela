---
id: RR-9S63SJ
type: review-response
title: '`render` is a config constant duplicated onto per-entity field data'
finding: '`SectionFieldData` (sections.go:56-62) carries per-entity facts — `Values`, `PropType`, `Inaccessible` all describe this entity''s value. `Render` is a config constant, identical for every row, so a cards/list section of N entities x M fields emits N*M copies of the same string. Review argued this is a category error and proposed emitting resolved render modes once on the section instead.'
severity: minor
resolution: Accepted as real but not worth the cost. The alternative (a parallel `FieldRenders []string` on the section, or section-only emission) forces the frontend to correlate section config with row fields by index — and `rowShouldRouteToInlineEdit`/`buildSectionEditFields` consume `row.fields`, not the section's, so an index-correlation would introduce a new coupling exactly where BUG-9QL9XV previously went wrong. Per-field emission keeps each row self-describing. Wire cost is a short enum string per field; typical sections are 3-10 rows. Documented as a deliberate tradeoff.
status: addressed
---

The observation is correct: `render` genuinely is config, not per-entity data.

Rejected because the cleaner shape costs more than it saves here.
`buildSectionEditFields` is called with `row.fields` for rows
(`sectionEditFields.ts:142`) and `section.fields` for the entry section
(`EntityDetail.vue:484`) — they are different arrays. Emitting render modes only
on the section would require the row path to index back into section config,
adding a correlation-by-position dependency between two structures that are
currently independent.

Self-describing rows are the safer default on a surface that has already had an
ACL bug (BUG-9QL9XV) caused by field data and config drifting apart.
