---
id: TKT-R6U15C
type: ticket
title: 'MCP attachment tools: list/read/attach/delete for agents, ACL-gated'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Give MCP agents the ability to work with attachments. Today `internal/mcp/` has
no attachment tool, so an agent can create/update entities but can neither
attach, read, nor remove a file.

### Scope
- Four MCP tools over `attachment.Service`: `list_attachments`, `read_attachment`, `attach_file`, `delete_attachment`. Follow the consumer-side interface pattern (CLAUDE.md) rather than widening `mcp.Deps.Store`.
- **ACL: inherit the owning entity's ACL**, consistent with the web and CLI paths. Reads use the gated reader (row, face, field); writes authorize `update` before any bytes are written.
- Same upload policy as the web path: MIME allowlist, scan/transform, per-property `max`, size limit.
- Both transports: stdio `rela mcp` and remote `rela-server -mcp`.

### Decisions (2026-09-25)
- Read returns bytes inline, capped at 10 MiB: images as image content, UTF-8 text as text, other types as a base64 blob.
- Upload takes base64 `content` only; no local-path input.

### Acceptance
- An agent can list, read, attach, and delete attachments through MCP.
- ACL is enforced (tests for hidden entity, hidden property, denied write).
- Honors `max`.

Parent: FEAT-870YCY. Plan: PLAN-DHSSFX.
