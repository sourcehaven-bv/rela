---
id: RR-RIWM94
type: review-response
title: Rename replaced the store count even without a gate
finding: With a nil gate the pre-Tx count overwrote the store's exact count and scanned relations twice.
severity: minor
resolution: The count runs and replaces the store's number only under a read gate.
status: addressed
---
