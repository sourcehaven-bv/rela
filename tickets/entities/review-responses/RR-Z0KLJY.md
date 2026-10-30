---
id: RR-Z0KLJY
type: review-response
title: Configure placeholder and preview disagree with the real busy header
finding: 'For property columns the label placeholder equals the header the list shows when label is empty. For a busy column the placeholder is "busy: run" (ListView.vue:172) and the preview header is columnTitle, "run" (ListView.vue:140, models.ts:706), while the list itself shows "Busy" (tableColumns.ts columnHeader). An operator who leaves the label empty expecting "busy: run" or "run" gets "Busy". The face column has the same mismatch ("face" vs "Face"). Fix: keep columnSource for the source text, but use the runtime default ("Busy", "Face") for the placeholder and the preview header, ideally by reusing columnHeader''s fallback.'
severity: minor
resolution: ADDRESSED ListView uses shownHeader, which reuses tableColumns columnHeader, for the label placeholder, the preview headers and the name label, so a busy or face column shows "Busy"/"Face" as the list does; columnSource still gives the source text; ListView.test.ts asserts the placeholder and the preview headers.
status: addressed
---
