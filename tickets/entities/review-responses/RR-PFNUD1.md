---
id: RR-PFNUD1
type: review-response
title: Unreadable data-entry.yaml lets acl audit pass the A15 gate
finding: auditWebhooks only warned when data-entry.yaml failed to parse, so --fail-on=medium exited 0.
severity: significant
resolution: With a --fail-on gate the audit returns an error; without one it warns. A missing file still declares no hooks. TestACLAudit_UnreadableDataEntry covers both.
status: addressed
---
