---
id: RR-6IL3X7
type: review-response
title: TraceResult.Properties aliases the store entity's live map — in-place redaction corrupts the store
finding: 'Verified: internal/tracer builds TraceResult with `Properties: e.Properties` (both traceFrom and traceTo paths) — a direct alias of the store entity''s map, not a copy. The plan''s phrase ''redact in-place on the decorator''s copied tree'' is dangerously ambiguous: the tree STRUCTS are fresh per call, but the Properties MAPS are shared with the store (memstore certainly; fsstore''s cached entity too). Deleting hidden keys in place would permanently mutate the live entity for every subsequent reader. The decorator must REPLACE each visible node''s Properties with a freshly built filtered map and must never call delete() on the aliased one. Conformance test must pin store-entity-unmutated after a redacted trace.'
severity: significant
resolution: 'Plan revised: decorator replaces each surviving node''s Properties with a freshly built filtered map; delete() on the aliased store map is forbidden by contract. Conformance case pins store-entity deep-equal before/after a redacted trace (PLAN-RR12W4 AC8).'
status: addressed
---
