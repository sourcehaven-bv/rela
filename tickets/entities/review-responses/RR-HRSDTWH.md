---
id: RR-HRSDTWH
type: review-response
title: New VerifiedFrom caller without a guard; stale docs
finding: forEachPrincipal is a new production caller of principal.VerifiedFrom. The godoc and the guide said only the JWT resolver calls it and no archguard test pins its callers.
severity: minor
resolution: 'The VerifiedFrom godoc names persisted verified claims as an exception and the guide describes the non-request callers. An archguard allowlist for VerifiedFrom callers is left to a follow-up: callers already exist in realtime and aclmap and cli and the allowlist is its own change.'
status: addressed
---
