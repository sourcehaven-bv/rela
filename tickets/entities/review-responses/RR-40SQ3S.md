---
id: RR-40SQ3S
type: review-response
title: Lua and MCP pile reads turn store faults into empty piles
finding: internal/lua/piles.go and internal/mcp/tools_piles.go use ResolveHeaders, which logs and treats every item as a miss on a header-read failure; show_pile/rela.piles.list then report an empty pile. HTTP uses ResolveHeadersErr.
severity: minor
resolution: Lua list/items and MCP list_piles/show_pile use ResolveHeadersErr and raise / return a tool error on a header-read failure. TestPiles_HeaderReadFailureRaises (lua), TestPiles_HeaderReadFailureIsToolError (mcp).
status: addressed
---
