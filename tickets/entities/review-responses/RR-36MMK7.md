---
id: RR-36MMK7
type: review-response
title: Verb.Valid accepts rename while the grant helpers answer it wrongly
finding: acl.Verb(rename).Valid() is now true, so AccessRoutes, EveryoneGrants and AssertedGrants answer rename from the update list. Only aclmap guards against it. Raised by both reviewers.
severity: minor
resolution: Verb.Valid godoc records that rename is valid but the report helpers answer it wrongly and that callers must exclude it until TKT-4APGVF. The only non-test callers are aclmap (excludes rename) and affordances (constants only).
status: addressed
---
