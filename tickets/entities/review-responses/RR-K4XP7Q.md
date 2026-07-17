---
id: RR-K4XP7Q
type: review-response
title: Nits — traceTitle 4-arg signature, GetEntity-isolation invariant
severity: nit
status: wont-fix
finding: (a) traceTitle(id, entityType, literalTitle, properties) has two adjacent string params — a swap-and-still-compiles footgun. (b) TraceResult.Properties aliases the map returned by store GetEntity; this is safe only because every store returns an isolated copy (memstore Clone, fsstore re-parse), an invariant asserted in a different package.
reason: (a) traceTitle has exactly one caller (writeTraceNode); path never calls it (PathStep.Properties removed per RR-MHYTYK), so the two-string-param hazard is theoretical. Taking *TraceResult would couple the helper to the tracer struct for no present benefit. Revisit only if a second caller appears. (b) The isolation invariant already holds for all in-tree stores and is exercised by the storetest conformance harness; no aliasing bug exists, and adding a defensive clone in the tracer would be premature. Documented the reliance in the TraceResult godoc.
---
