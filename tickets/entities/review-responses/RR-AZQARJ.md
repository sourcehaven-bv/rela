---
id: RR-AZQARJ
type: review-response
title: 'Code: MCP hot reload drops options'
finding: mcp_wiring rebuilds SharedBase with only WithACL.
severity: minor
reason: rela mcp runs foreground, so nothing is lost today; carrying options on reload is a separate change.
status: deferred
---
