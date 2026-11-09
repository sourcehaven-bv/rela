---
id: RR-1BC0DZ0
type: review-response
title: Partial erasure is not reported
finding: When the drop succeeds and the audit removal fails, only "erase failed" is printed.
severity: minor
resolution: The error now says the schema was dropped and that rerunning is safe; the erasure record carries schema_dropped. Pinned by TestErase_AuditFailures.
status: addressed
---
