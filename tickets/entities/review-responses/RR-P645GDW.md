---
id: RR-P645GDW
type: review-response
title: Loaded rows are not gated after the header gate
finding: servedIDsErr gates headers, then loadRows reads the rows again ungated; a row that became unreadable in between would be sent to the model.
severity: minor
resolution: batchRows re-gates the loaded rows with visibleReader.filterVisible (one probe per type) before batchActionRows.
status: addressed
---
