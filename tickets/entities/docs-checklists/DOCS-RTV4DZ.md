---
id: DOCS-RTV4DZ
type: docs-checklist
title: 'Docs: MCP attachment tools'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (the snapshot-per-call rule, the preflight order, the separator refusal, the MIME sniff fallback, the body limit)
- [x] Function/type docs if public API (AttachmentDeps, AttachmentSnapshot, NewAttachmentSnapshot, MaxUploadBytes, MaxInlineReadBytes, dataentry.MCPHost, attachment.EntityPatcher, DetachFile, RejectionReason, audit.AttachmentWriteDenied/AttachmentRejected)

## Project Documentation

- [x] ~~README updated~~ (N/A: README lists no individual MCP tools)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; the change follows existing consumer-side interface, PatchEntity and visibility rules)
- [x] Help text accurate (rela detach now reports when a named file was not attached)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog; release notes come from commits)
- [x] API docs updated (GUIDE-mcp-server: Attachment Tools section and remote request-size note; GUIDE-attachment-security: MCP as an ingress under the same policy; docs/ regenerated)
