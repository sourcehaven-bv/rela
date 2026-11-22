---
id: RR-Q2SLOIG
type: review-response
title: Refusal errors carry no reason
finding: ErrPropsNotPushable reaches the user as a tool error; wrap it with a reason and fix its doc
severity: minor
resolution: LowerConditionExplained returns UnsupportedConditionError naming the construct (rendered source text) and a reason; the MCP tool prefixes it with the supported shapes. Tests in queryplan, predicate, appbuild and mcp.
status: addressed
---
