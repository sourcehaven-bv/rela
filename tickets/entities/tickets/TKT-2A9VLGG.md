---
id: TKT-2A9VLGG
type: ticket
title: Remote MCP toggle and tool filter in .rela/config.yaml
kind: enhancement
priority: high
effort: s
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

Operators need to switch rela-server's remote MCP endpoint (`/api/v1/_mcp`) on
and off in operator config, not only with the `-mcp` flag or `RELA_MCP=1`. An
internal instance such as a billing back office must be able to refuse remote
MCP even if someone later adds the flag or variable to its environment.
Operators also need to narrow the tools the endpoint serves, for example to a
read-only set.

## Approach

- `remote_mcp:` block in `.rela/config.yaml`. That file already holds the
deployment setting `validation:`, is not committed, and lets two deployments of
one schema differ.
- `remote_mcp.enabled` (true/false/unset) combines with `-mcp` and
`RELA_MCP`: the endpoint is served only when no source says off and at least one
says on. An off wins and is logged when it overrules an on.
- `-mcp=false` and `RELA_MCP=0` now say off. Any other `RELA_MCP` value than
`1`, `0` or empty stops startup.
- `remote_mcp.tools.allow` / `deny` become `mcp.WithToolFilter`. A filtered
tool is not registered and cannot be replayed by `watch`. An unknown name stops
startup. The block is decoded strictly, so a misspelled key stops startup too.

## Acceptance criteria

- [ ] Every flag/env/config combination resolves by the rule above (tested).
- [ ] Allow, deny and deny-over-allow filter `tools/list` and calls (tested).
- [ ] Unknown tool names and misspelled config keys stop startup (tested).
- [ ] docs/mcp-server.md and docs/server-security.md describe the precedence.
