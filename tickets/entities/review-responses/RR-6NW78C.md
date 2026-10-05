---
id: RR-6NW78C
type: review-response
title: Resolver parity test deleted
finding: resolver_parity_test.go compared visibility.Resolver.Address against getVisibleRef. The PR deleted it.
severity: minor
reason: The test compared the resolver with getVisibleRef. This PR deletes getVisibleRef so there is nothing left to compare. The gate branches it walked are covered by the resolver's own tests in internal/visibility and by the handler tests here (TestRelationWrites_DeniedFaceIsTheUniformMiss and the entity GET ACL tests).
status: wont-fix
---
