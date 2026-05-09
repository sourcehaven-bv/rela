---
id: RR-X5WVA
type: review-response
title: RelationOptions audit list is aspirational, not enumerated
finding: |-
    Plan defers call-site audit to implementation. Reviewer enumerated 9 sites: handlers_api.go:527, relations.go:113, api_v1.go:730, api_v1.go:772, manager.go:138, manager.go:147, mcp/tools_relation.go:76, cli/link.go:27, lua/runtime.go:1373, plus PanicOnUse stub. Two are public APIs (MCP, Lua). Recommendation: enumerate in plan now; for each state new value and behavior; flag MCP/Lua for behavioral notes.

    From design-review: F13.
severity: minor
resolution: 'Plan Layer 0 includes the full call-site enumeration: 9 sites + 1 stub, with current call, new call, and behavioral-change column for each. MCP and Lua flagged as having subtle behavior changes.'
status: addressed
---
