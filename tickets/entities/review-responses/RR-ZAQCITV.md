---
id: RR-ZAQCITV
type: review-response
title: Admission calls grow with rows scanned
finding: listAdmitted flushed every 32 rows while pages grew to 256, so a long scan ran q.Admit (one MatchingFaces per type) every 32 rows.
severity: significant
resolution: The batch threshold now grows with the page size (nextPageSize), so a scan admits in O(log + N/256) calls.
status: addressed
---
