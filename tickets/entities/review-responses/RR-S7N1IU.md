---
id: RR-S7N1IU
type: review-response
title: A primed ctx could reach a write authorization
finding: PrimeLocalRoles stores a snapshot on ctx. A write that inherits a primed ctx would authorize on roles computed before earlier writes in the same request.
severity: minor
resolution: acl.WithoutLocalRolePrime strips the prime; entitymanager.authorizeAndAudit applies it to every real write (affordance probes keep it). TestWithoutLocalRolePrime_SeesLaterEdges pins it.
status: addressed
---
