---
id: RR-WMKWDC2
type: review-response
title: Type-assertion back-channel for the audit log in the CLI
finding: The CLI reached the sql log by asserting on the Audit value, which is the interface{} back-channel the rules forbid.
severity: significant
resolution: Services.AuditLog returns the typed log, and the CLI declares a narrow auditLog interface at the call site.
status: addressed
---
