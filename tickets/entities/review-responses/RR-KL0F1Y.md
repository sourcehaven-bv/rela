---
id: RR-KL0F1Y
type: review-response
title: Delete prompts and results do not mention owned entities
finding: CLI, MCP and SPA said only Delete X? and reported a relation count.
severity: minor
resolution: CLI and MCP report the owned count; the SPA confirm says owned entities are deleted too. Tests in cli, mcp and EntityDetail.owner.test.ts.
status: addressed
---
