---
id: RR-R9ZMYM
type: review-response
title: 401 leaves the user on a blank page with no way to re-authenticate
finding: 'jwtgate.go is the only 401 producer and fires from middleware across the whole API, but the SPA has no global 401 handling — client.ts has one interceptor that only normalizes the error, and there is no login redirect anywhere. Blanking on 401 therefore converts a silent-staleness bug into a visible dead end: blank view, toast, and a Refresh button that 401s again.'
severity: significant
reason: 'The gap predates this change and is out of scope for a fix to the document loaders. It is an absent global feature (a re-authentication path for the whole SPA), not a defect in the branch under review, and building it here would mean designing login-redirect behaviour for deployments that may run without auth at all — several rela deployments do. Confidentiality still wins the immediate trade: a blank view with no recovery is better than content the principal may no longer read. Recorded in the bug entity''s ''Known gap this exposes'' section and filed as a follow-up, which carries the constraint that the handler must key off err.status === 401 directly and never off shouldDropHeldContent, since a 404 is a true result for that predicate.'
status: deferred
---

Deferred rather than dismissed. The reviewer's framing is right that the fix
makes a pre-existing gap newly visible, and that is a reason to name it
explicitly rather than let it be discovered in production — which is what the
bug entity and the follow-up ticket now do.
