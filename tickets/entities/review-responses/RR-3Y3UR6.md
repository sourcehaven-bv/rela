---
id: RR-3Y3UR6
type: review-response
title: _world_absent_name is shipped on the wire and typed on the client with no consumer left
finding: The only consumer (EntityDetail worldAbsentName) was removed. internal/apiwire/v1/responses.go still computes and ships `_world_absent_name`, frontend/src/api/views.ts still types it, and one test fixture sets it without asserting. Either document a consumer or remove it deliberately.
severity: significant
resolution: 'Removed end to end: the wire field, its producer in viewworld.go, the client type, the test fixture and the data-entry guide''s mention. It was introduced on the still-unmerged TKT-SLFURL branch, so no released API carried it.'
status: addressed
---
