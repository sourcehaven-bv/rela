---
id: RR-308FSL
type: review-response
title: World resolved per read, not per operation
finding: Each read called the source, so a reload mid-call could mix worlds and the grant check ran many times.
severity: minor
resolution: withMCPWorldMemo resolves the world once per MCP request. TestMCPReadWorld_ResolvesOncePerRequest.
status: addressed
---
