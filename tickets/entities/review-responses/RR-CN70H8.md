---
id: RR-CN70H8
type: review-response
title: Processor-rejected MCP uploads not audited
finding: 'Web path audits attachment.ErrRejected as denied-write (TKT-6O8D0L, CONTROL-8-15); plan only audits ACL denies. Fix: share the record construction in one helper used by both ingresses; test that a disallowed MIME upload yields one denied-write record.'
severity: significant
resolution: writeError records audit.AttachmentRejected (op=denied-write, op=attachment-write) on ErrRejected. TestAttachments_RejectedUploadIsAudited; verified manually over stdio.
status: addressed
---
