---
id: RR-XD0AP
type: review-response
title: listRelations swallows read errors
finding: services.go listRelations drops iterator errors and returns a partial slice. reconcileOutgoingRelations then treats the missing edges as absent and issues CreateRelation which fails with 'relation already exists'. Either propagate errors from listRelations, or have reconcile call a variant that does.
severity: significant
resolution: listRelations now has a sibling listRelationsCtx that returns (slice, error) and the read path used by reconcile (outgoingRelationsCtx) goes through it so iterator errors propagate. The old listRelations is kept as a thin background-ctx wrapper for existing call sites that can't meaningfully handle the error.
status: addressed
---
