---
id: RR-VFFS9M
type: review-response
title: '[security] PATCH/DELETE on a faced address disclosed whether a denied face exists (403 vs 404)'
finding: '[security] Both write handlers row-gated the bare id and then loaded the row through the ungated entityReader without applying the face gate, so a face the principal may not read reached the write ACL and answered 403 (naming the rule) while an absent face answered 404. The status code alone revealed which content states exist — the existence a `type@face` read grant withholds. The same path also returned the PATCH response body of a face the read grant hides.'
severity: significant
resolution: handleV1UpdateEntity and handleV1DeleteEntity now apply faceReadable to the loaded row before authorization and answer the uniform not-found (entityNotFoundTitle) on denial. TestFacedAddress_WritesDenyAsNotFound pins PATCH and DELETE on a denied-but-existing face against a missing face, comparing the problem bodies minus the echoed instance path.
status: addressed
---
