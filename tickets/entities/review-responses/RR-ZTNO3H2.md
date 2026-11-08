---
id: RR-ZTNO3H2
type: review-response
title: t.as roles are IdP claims mapped via asserted_role_assignments so expect_denied passes vacuously
finding: 'acl/resolver.go:46-63: claim values only select asserted_role_assignments entries; unknown claims are dropped.'
severity: critical
resolution: 't.as(user) names a principal only; roles come from the real policy plus fixture user/group entities. t.principal{roles} creates a user entity and a per-case policy clone with assignments passed via WithACL (shared policy never mutated). New AC: a positive control write must succeed before a denial assertion counts.'
status: addressed
---
