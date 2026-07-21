---
id: RR-R2DG19
type: review-response
title: Pin that the wire serializer intentionally drops the doc-fields (phase boundary)
finding: toV1CustomType (dataentry/api_v1.go) projects only Values/Labels/Default and drops Descriptions; nothing carries Transitions/Help to the wire. Correct for phase 1a (a FUTURE generator renders these), but the fields are inert today. A future dev might 'helpfully' wire Descriptions to the wire without the frontend contract change dataentry/CLAUDE.md requires. Recommend a small test pinning the intentional omission so the phase boundary is explicit.
severity: minor
resolution: Documented the intentional omission on toV1CustomType (the single wire-serialization site) and added TestToV1CustomType_OmitsDocFields pinning that Descriptions/Transitions/Help never reach the wire (v1.CustomType has no field for them; marshaled JSON asserted free of those keys). A future wiring must intentionally update the test + the frontend contract. Also added a rename-path round-trip test (RR#5) asserting the doc-fields survive RenameEntityType's AST re-marshal.
status: addressed
---
