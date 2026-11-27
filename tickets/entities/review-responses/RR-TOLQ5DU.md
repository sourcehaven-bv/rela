---
id: RR-TOLQ5DU
type: review-response
title: Offset with in-type row gating
finding: 'Security review: q.Offset on the first type read must count only rows the principal may read, or hidden rows shift pages.'
severity: minor
resolution: The offset is applied inside the ACL-scoped query; TestSearchPage_HiddenRowsDoNotShiftPages and the differential test with the alice principal pin it.
status: addressed
---
