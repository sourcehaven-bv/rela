---
id: RR-XSF73A
type: review-response
title: MCP watch long-poll not listed in scope
finding: The scope covers the two event streams and excludes command-exec. It does not mention the remote MCP watch tool (internal/mcp/tools_watch.go). That tool is served on /api/v1/_mcp behind the same gate. watch blocks for up to watchMaxTimeout (50 s) and replays reads as the caller, so it can also outlive the token. The bound makes this acceptable, but the plan sets out to list every place that outlives the token. Add watch to the "Out" list with the 50 s bound as the reason, so that leaving it out is a recorded decision.
severity: minor
resolution: 'Plan updated: MCP watch listed as out of scope; each call replays for at most 50 s and the next call goes through the gate.'
status: addressed
---
