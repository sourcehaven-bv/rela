---
id: RR-1FP07FP
type: review-response
title: 'Missed consumers: MCP reload, Configure preview, schema.Analyze'
finding: rela mcp watches schema/realtime/acl but not flows.yaml; Configure preview compiles automations via CheckSchemaCompiles; schema.Analyze counts automation references used by cleanup.
severity: significant
resolution: 'Plan: add a flows.yaml watch to MCP reload; Configure Validate parses flows.yaml against the candidate schema; Analyze takes the trigger rules.'
status: addressed
---
