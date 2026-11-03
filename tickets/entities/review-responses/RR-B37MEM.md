---
id: RR-B37MEM
type: review-response
title: Docs claim every answer leads with notice, and that signals need signals:true to wake
finding: 'docs/mcp-server.md:492: signalResult and error texts have no notice. docs/mcp-server.md:511: signals default to true for a target without a cursor, so a newest-page watch does wake on typing signals on remote.'
severity: nit
resolution: 'The MCP guide now says notice leads only the answers that carry untrusted content, so signal-post answers and error texts have none. Signals follow the read_channel default: a target without a cursor includes them and wakes on them. The guide also covers the follower cap, page targets and the acl.yaml reload and refusal. The realtime guide wording is fixed the same way. Regenerated; docs-check is clean.'
status: addressed
---
