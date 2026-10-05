---
id: RR-UHQFT5
type: review-response
title: FaceMatcher declared in both store and acl
finding: store.FaceMatcher (for the MatchingIDs/IDMatcher helpers) and acl.FaceMatcher are the same one-method shape.
severity: nit
reason: 'Consumer-side interfaces per the project rule: each package declares the narrow interface it consumes.'
status: wont-fix
---
