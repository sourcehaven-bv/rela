---
id: RR-JZFW0
type: review-response
title: Dead caching machinery in analyze.go
finding: resolveAnalyzeOpts/caching exists for a function that always returns empty options
severity: significant
resolution: Removed cachedOpts/cachedOptsOnce/resetAnalyzeOptsCache/doResolveAnalyzeOpts, simplified resolveAnalyzeOpts to return empty options directly
status: addressed
---
