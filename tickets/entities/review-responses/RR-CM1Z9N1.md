---
id: RR-CM1Z9N1
type: review-response
title: Import is not atomic and can plant maintenance records
finding: A failing import left a partial append and imported audit-prune records could vouch for a forged anchor.
severity: significant
resolution: Import runs in one transaction, refuses audit-prune and audit-import records, and appends rows at the record time. Pinned by ImportStopsAtError and ImportRefusesMaintenanceRecords.
status: addressed
---
