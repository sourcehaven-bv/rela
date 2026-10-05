---
id: RR-JPDD6K
type: review-response
title: MCP coverage constants duplicate analysis.Coverage and type filter ran after ResolveHeaders
finding: internal/mcp/tools_analysis.go copies the coverage strings and resolved headers for orphans the type filter then dropped.
severity: minor
resolution: 'The type filter now runs before ResolveHeaders. The constants stay: arch-lint keeps mcp off internal/analysis, and each side is pinned by its own tests.'
status: addressed
---
