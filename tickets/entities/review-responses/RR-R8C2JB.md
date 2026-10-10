---
id: RR-R8C2JB
type: review-response
title: Rejected-upload audit records the raw file name
finding: auditRejectedUpload logged the client's raw name; a 1 MB query-string name would bloat the append-only log.
severity: minor
resolution: Audit uses attachment.DisplayName (normalized and length-capped). TestAttachmentUpload_RejectionAuditsNormalizedName.
status: addressed
---
