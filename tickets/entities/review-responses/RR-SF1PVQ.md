---
id: RR-SF1PVQ
type: review-response
title: Gate failures logged at Error on MCP, Warn on HTTP
finding: opError logged every 5xx at Error, while writeGateError logs gate failures at Warn
severity: minor
resolution: 'Fixed: commentFailure carries warn, set by failGate; opError logs at that level.'
status: addressed
---
