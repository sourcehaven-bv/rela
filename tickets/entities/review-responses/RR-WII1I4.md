---
id: RR-WII1I4
type: review-response
title: A caller without the world grant is locked out of MCP
finding: A denied app.default_world made every MCP read and write-existence check return not found; MCP has no ?world=default escape.
severity: significant
resolution: A denied world now falls back to the default world, which needs no grant; row and face gates still apply. Binding.Denied removed; Source returns store.WorldScope. TestMCPReadWorld_ThroughTheRouter covers both cases with the real ACL gate.
status: addressed
---
