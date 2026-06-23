---
id: TKT-R6U15C
type: ticket
title: 'MCP attachment tool: attach/list/read for agents, ACL-gated'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Give MCP agents the ability to work with attachments. Today `internal/mcp/` has
no attachment tool, so an agent can create/update entities but can neither
attach a file nor read one back.

### Scope
- MCP tool(s) over the existing `store.AttachmentManager` / `attachment.Service`: list attachments for an entity, read an attachment's bytes (or a reference the client can fetch), and attach a file. Follow the consumer-side `mcp.Services` interface pattern (CLAUDE.md) rather than widening a god interface.
- **ACL: inherit the owning entity's ACL** — consistent with the web and CLI paths. An agent that can't read the entity can't read its attachment.
- Respect the default size limit (shared write-path enforcement from TKT-RXFD5B).

### Acceptance
- An agent can list, attach, and read attachments through MCP.
- ACL is enforced (test).
- Honors `max` once TKT-WLLRO7 lands (build the tool tolerant of N-per-property).

### Notes
- Secondary priority — sequence after the web path. Reading binary bytes over MCP may warrant returning a fetchable reference rather than inlining large blobs; decide during planning.

Parent: FEAT-870YCY.
