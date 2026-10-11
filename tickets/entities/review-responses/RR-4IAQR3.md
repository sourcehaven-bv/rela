---
id: RR-4IAQR3
type: review-response
title: Raw body over MCP is a re-serialization
finding: rela.request.raw over MCP is re-marshalled JSON, not the caller's bytes, and the cap applies after decode.
severity: minor
resolution: Documented on RunAction. The MCP mount has its own request size limit (22.3 MiB, docs/mcp-server.md), so decode memory is bounded there.
status: addressed
---
