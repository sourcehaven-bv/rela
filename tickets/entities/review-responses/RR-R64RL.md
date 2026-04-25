---
id: RR-R64RL
type: review-response
title: MCP test substring match too loose
finding: TestHandleCreateEntity_RejectsCustomIDForShortType asserted on 'requirement', 'short', 'my-custom-id'. 'short' substring-matches unrelated messages. A refactor that stopped naming the caller's ID input could still pass.
severity: minor
resolution: Added 'custom ID' to the assertion set in both the MCP handler test and the workspace-level negative tests. Pins that the error complains about the caller's ID input specifically.
status: addressed
---
