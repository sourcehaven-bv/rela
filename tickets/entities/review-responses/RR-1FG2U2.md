---
id: RR-1FG2U2
type: review-response
title: Non-HTTP comments transport disagrees with the HTTP status
finding: failACLDenied and failGate report forbidden or acl_query_failed to the non-HTTP transport while HTTP answers 401. Raised by both reviewers.
severity: minor
resolution: failNoIdentity builds the unauthenticated failure; failGate, failInternal and failACLDenied use it. A mismatch in failACLDenied is acl_identity_mismatch.
status: addressed
---
