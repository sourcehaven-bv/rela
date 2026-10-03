---
id: RR-EUKDA9
type: review-response
title: computeDocumentHash hides the real error
finding: Every loadEntry error (parse failure, store outage) becomes entity not found; wrap with %w.
severity: minor
resolution: computeDocumentHash wraps the load error with %w.
status: addressed
---
