---
id: RR-ZARZTR
type: review-response
title: Security-critical write preflight duplicated across ingresses
finding: 'MCP would re-implement attachmentWritePreflight''s audit record shapes. Fix: hoist the denied-write and rejected-upload record construction into one shared helper used by web and MCP.'
severity: minor
resolution: Record construction hoisted into internal/audit/attachment.go (AttachmentWriteDenied, AttachmentRejected) plus attachment.RejectionReason; the web handler and MCP both use them.
status: addressed
---
