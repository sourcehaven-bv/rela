---
id: RR-C63Q6U
type: review-response
title: Per-face cardinality and MCP unique checks untested
finding: The data-entry per-face cardinalityBound path and MCP handleAnalyzeUnique had no tests.
severity: significant
resolution: Added the 'cardinality per face' subtest to TestAnalyze_FacedTypes (every face and published-only) and TestAnalyzeUnique_PerFace in internal/mcp (per face, hidden face absent). MCP unique output is now sorted.
status: addressed
---
