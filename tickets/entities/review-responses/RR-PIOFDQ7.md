---
id: RR-PIOFDQ7
type: review-response
title: expect_access disagreed with the write path under principal_property
finding: aclmap mapped a declared principal by its raw name while the write path looked it up through principal_property, so expect_access judged a different principal than the actor.
severity: significant
resolution: aclmap.MapPrincipalFull resolves the name through the lookup, and actors bind as the principal's assignee. TestPrincipalNaturalKey covers both.
status: addressed
---
