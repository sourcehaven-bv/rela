---
id: RR-KH35J1
type: review-response
title: Bare-id update_entity on a faced entity fails as not found
finding: GetEntity resolved the world prime but PatchEntity addressed the missing default row.
severity: significant
resolution: faceAddressRequired returns an error naming ID@face when a bare id resolves to a named face; update_entity and delete_entity descriptions updated; golden regenerated.
status: addressed
---
