---
id: RR-9Q0DUD
type: review-response
title: Trace JSON leaked full property map on every node
severity: critical
status: addressed
finding: Adding Properties to TraceResult/PathStep (serialized directly via writeJSON, structs have no JSON tags) added a Properties blob to every node/step in `rela trace --format json` output — unbounded (unlike the 40-char truncated text), a schema change nobody asked for, and backwards from the TKT-VHSHOB "JSON stays raw" convention this builds on. The MCP path dodged it via a dedicated DTO (convert.go); the CLI dumped the tracer struct as-is.
resolution: Tagged TraceResult.Properties `json:"-"` so it carries data internally for text resolution but never serializes. Removed the Properties field from PathStep entirely (path text renders only ID/Type, never a title — see RR-MHYTYK). Verified `rela trace -o json` no longer emits Properties; added an assertion to TestWriteTraceJSON that pins Properties (and a canary value) absent from the trace JSON.
---
