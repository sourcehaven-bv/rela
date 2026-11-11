---
id: RR-5GR71V3
type: review-response
title: Stdio MCP flow tools stay on the startup assembly
finding: mcpwire.Flows(svc.current()) captured the startup entity manager, so after a schema.yaml reload complete_flow_step validated against the old metamodel.
severity: significant
resolution: rela mcp now uses mcpwire.CurrentFlows(svc.current), which resolves the flow control per call. TestCurrentFlows_ResolvesPerCall pins it.
status: addressed
---
