---
id: RR-NXYFUR5
type: review-response
title: MCP payload cannot carry 4 x 10 MiB
finding: MCP request bodies are capped near base64(16 MiB)+1 MiB.
severity: significant
resolution: Total decoded bytes per message are capped at 16 MiB (= mcp.MaxUploadBytes) on every transport; the MCP tool shares attachment.Limiter.
status: addressed
---
