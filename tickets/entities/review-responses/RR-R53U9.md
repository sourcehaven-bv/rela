---
id: RR-R53U9
type: review-response
title: MCP tool result loses isError signal that AI clients use to detect failure
finding: 'Plan says ''AI agents see Warnings: section in result text''. Wishful. MCP isError:true is a structured signal an AI client can branch on without parsing body. With this ticket, validation failures become success-with-warnings text. Agents doing ''if result.isError { retry }'' silently get success. Plan''s mitigation ''agents read the result'' relies on every prompt being engineered to scan for ''Warnings:''. Recommendation: (a) attach JSON _meta block with warnings.length so agents can branch programmatically, OR (b) keep isError:false (write succeeded — that''s true) but make Warnings the FIRST thing in result text in fixed format with WARNING: sentinel lines, document in MCP tool description so agents are primed. Add integration test that warnings are programmatically detectable, not just substring-present. From design-review F6.'
severity: significant
resolution: MCP tool result begins with 'WARNINGS (n):' prefix as the leading section (AC18). Tool description (AC20) explicitly documents the convention so AI agents are primed to look for it. isError stays false (write succeeded — that's the truth). AC18 asserts programmatic discoverability, not just substring presence. Layer 5 spec includes the exact format string and the description text to register with MCP.
status: addressed
---
