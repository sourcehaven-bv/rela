---
id: RR-WO03W25
type: review-response
title: remote_mcp mistake fails every config.yaml reader
finding: validationModeFor also loads .rela/config.yaml, so a typo in remote_mcp breaks the CLI, stdio MCP and desktop too.
severity: minor
resolution: Kept deliberately (fail closed) and documented in GUIDE-mcp-server and the RemoteMCPConfig godoc.
status: addressed
---
