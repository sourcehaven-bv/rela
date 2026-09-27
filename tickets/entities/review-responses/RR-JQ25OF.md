---
id: RR-JQ25OF
type: review-response
title: Remote MCP freezes attachment policy at boot
finding: 'Web path rebuilds PolicyProcessor and limit from the current schema per request; the remote MCP factory runs once. Fix: MCPHostDeps carries a provider backed by the App''s schema snapshot, read once per call.'
severity: significant
resolution: AttachmentDeps.Snapshot is resolved once per tool call; remote wiring builds it from the App's live schema (MCPHost.AttachmentPolicy) so accept/scan/max_attachment_bytes edits apply immediately.
status: addressed
---
