---
id: RR-JA14MOI
type: review-response
title: Erasure leaves no durable record
finding: The only trace of an erasure was a log line, and the tenant's audit log is what gets deleted.
severity: minor
resolution: Each attempt is appended to .rela/audit/tenant-erasures.jsonl (time, operator, org, schema, result; no tenant data). Pinned in TestEraseTenant_EndToEnd and TestEraseTenant_NeedsPostgres.
status: addressed
---
