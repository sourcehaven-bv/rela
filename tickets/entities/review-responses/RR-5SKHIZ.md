---
id: RR-5SKHIZ
type: review-response
title: Lua and MCP adds resolve each id with a separate read
finding: lua/piles.go and mcp/tools_piles.go call WriteTarget per address, so 500 ids make 500 sequential gated reads; the HTTP path batches with one ResolveHeadersErr.
severity: minor
resolution: Lua pileWriteRefs and MCP writeRefs batch-resolve with one ResolveHeadersErr and call WriteTarget only for unsettled (bare faced) addresses. TestPiles_AddReadBudget in lua and mcp pins equal reads at 10 and 50 ids.
status: addressed
---
