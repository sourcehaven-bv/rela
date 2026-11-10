---
id: RR-5WJH5ZH
type: review-response
title: MCP flow tools return raw internal errors
finding: errorResult(err.Error()) passed store and engine error text to the client.
severity: nit
resolution: flowErrorResult passes on only errors with a flowview kind; others are logged and answered with a fixed text.
status: addressed
---
