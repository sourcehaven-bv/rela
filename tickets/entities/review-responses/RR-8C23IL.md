---
id: RR-8C23IL
type: review-response
title: Lua render override on export must reuse the gated _documents path, not call ExecuteDocument fresh
finding: 'An existing authorized endpoint GET /api/v1/_documents/{docName}/{entityId} (handleV1Documents) runs a Lua document render but gates it: gateReadOrNotFound BEFORE any render, entity_type enforcement, path-segment cache-escape guards (TKT-C0R07J). If the export endpoint invokes script.ExecuteDocument directly for a Lua render override, it bypasses these and opens an unauthorized Lua-on-read hole.'
severity: critical
resolution: 'Export routes the Lua render override THROUGH the existing gated document render path: gate -> documentService.Render -> ExecuteDocument, producing markdown/HTML, then pass that output to the transform. The export endpoint does not call ExecuteDocument on a fresh surface. If a direct call is ever unavoidable, it must replicate gateReadOrNotFound + entity_type enforcement + path-segment guards verbatim. Documented as a load-bearing invariant in the plan.'
status: addressed
---
