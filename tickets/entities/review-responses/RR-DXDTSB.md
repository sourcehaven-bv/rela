---
id: RR-DXDTSB
type: review-response
title: buildListQuery re-derives the pre-RR-6RF60V filter-key bug (filter[status][ne] keys on "status][ne")
finding: 'buildListQuery parsed filter[...] query keys with a naive CutPrefix/TrimSuffix pair. That is the exact shape RR-6RF60V already fixed elsewhere: an operator segment is swallowed into the property name, so filter[status][ne]=done keys the script''s rela.document.query.filters table on "status][ne" instead of "status". A render override would therefore report a filter name that never matches what the pipeline actually filtered on.'
severity: significant
resolution: Routed through the existing parseRelationFilterKey helper (internal/dataentry/api_v1.go), which uses the same ][-split logic as applyV1Filters and was written to fix precisely this bug. The operator segment is intentionally dropped (this context reports WHICH properties were filtered) and last-value-wins now matches applyV1Filters. Pinned by TestExport_List_RenderOverride_FilterOperatorKey, whose sensitivity was verified by temporarily reverting the fix and confirming the test fails with map[status][ne:done].
status: addressed
---
