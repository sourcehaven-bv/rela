---
id: RR-N6KZJNI
type: review-response
title: MCP list_worlds has no labels
finding: internal/mcp/tools_world.go omits label, so an agent cannot map a label to a world name.
severity: minor
resolution: 'Addressed in plan: list_worlds reports label and hidden; it does not filter hidden worlds.'
status: addressed
---
