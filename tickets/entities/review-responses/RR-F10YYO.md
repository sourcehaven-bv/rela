---
id: RR-F10YYO
type: review-response
title: Chunked watch request holds an upload slot
finding: 'Cranky review: on remote MCP a request without Content-Length counts as large and holds one of two upload slots while it waits.'
severity: nit
resolution: 'Documented in docs/mcp-server.md: send a Content-Length header. Clients normally do.'
status: addressed
---
