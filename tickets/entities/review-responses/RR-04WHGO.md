---
id: RR-04WHGO
type: review-response
title: Concurrent create race dropped requested relation properties
finding: When a create key already existed because of a race, the requested relation properties were silently dropped.
severity: minor
resolution: Fixed in 21e383051. An existing create key returns ErrRelationAlreadyExists. The handler maps it to 409 so the client retries as an update. Covered by TestReplaceRelations_ExistingCreateIsAlreadyExists.
status: addressed
---

Review finding R1-8.
