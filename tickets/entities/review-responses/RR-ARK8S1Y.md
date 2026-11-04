---
id: RR-ARK8S1Y
type: review-response
title: One save replaced the view once per property in the response
finding: mergeServerResponse reports every property of the row, so each save rebuilt viewData and the row index many times and called invalidate many times.
severity: minor
resolution: applyPropertyToTableRow returns the same sections for a value the row already holds; the handler then does nothing and does not invalidate. Pinned in sectionTableRows.test.ts.
status: addressed
---
