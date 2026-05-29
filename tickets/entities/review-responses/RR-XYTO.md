---
id: RR-XYTO
type: review-response
title: Relation-meta-field path untested; partial-compile is latent fail-open (S4)
finding: relation_accum.go candidateMeta/allowMeta/denyMeta at 0% coverage; metaFieldResults at 33%; no test exercises RelationGrant.Fields meta grants. The 'meta field can be more restrictive than its grant' AND-logic ships unverified. compileRelationGrant appends the grant when err==nil even if a meta-field predicate failed (skipped via continue) — harmless today since New hard-fails on any error, but a latent fail-open if New ever relaxes to warn-and-continue.
severity: significant
resolution: 'Added TestResolver_RelationMetaField exercising RelationGrant.Fields with a conditional when (AND-ed with grant): status=done denies note meta, status=open allows it. compileRelationGrant now tracks metaFailed and appends the grant only when err==nil AND !metaFailed — a grant that lost a meta field to a compile error is no longer half-installed. Affordances coverage rose to 84.6%.'
status: addressed
---
