---
id: RR-M6KO5T
type: review-response
title: Re-creating the identical owning edge returns 422 instead of a conflict
finding: checkOwningEdge counted the identical edge as an existing owner.
severity: minor
resolution: The identical edge is skipped so the store reports the conflict. TestCreateRelation_IdenticalOwningEdgeIsAConflict.
status: addressed
---
