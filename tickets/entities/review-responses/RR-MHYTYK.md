---
id: RR-MHYTYK
type: review-response
title: PathStep.Properties was dead weight (path text renders no title)
severity: significant
status: addressed
finding: WritePath renders each step as `ID (Type)` and nothing else — no title, resolved or literal. So PathStep.Title was already unused by text output, and the newly-added PathStep.Properties was populated at three sites for zero rendering benefit — read by nobody (not WritePath, not MCP), only surfacing as the JSON bloat from RR-9Q0DUD.
resolution: Removed the Properties field from PathStep and dropped it from all three construction sites. Kept Properties only on TraceResult, which is the sole struct whose text rendering (writeTraceNode) resolves a display title. Path output deliberately shows ID/Type only — unchanged.
---
