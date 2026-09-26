---
id: RR-9DDPMV
type: review-response
title: Document that stdio ignores max_attachment_bytes
finding: 'Stdio uses the store backstop like rela attach. Fix: state in docs that the configured limit applies to the remote transport.'
severity: nit
resolution: docs/mcp-server.md (Attachment Tools > Stdio differences) states that max_attachment_bytes applies only to remote and that scan/command transforms reject stdio uploads.
status: addressed
---
