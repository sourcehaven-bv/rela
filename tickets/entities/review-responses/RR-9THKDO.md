---
id: RR-9THKDO
type: review-response
title: No reliable per-request 'is JWT' signal + ambiguous 'unmatched' predicate
finding: 'S3: the helper is meant to no-op unless the principal is a verified-JWT principal, but a JWT principal and a --principal-header principal both stamp Tool=ToolDataEntry, and org/roles are legitimately empty for a JWT with no such claims — so there is no per-Principal discriminator. The only trustworthy ''JWT active'' fact is construction-time a.jwtGate != nil (wiring state), not anything on the Principal. S4: ''unmatched'' is inferred from User==sub / RawUser=='''', but that''s also true for every header/env principal AND for the lookup-disabled case (ResolvePrincipal returns ('''',nil) unconditionally when principal_property unset, declarative.go:128). So `provision` without principal_property would treat EVERY request as unmatched and provision a stub keyed on nothing. Need: (1) derive the guard from a.jwtGate!=nil, not the Principal; (2) a load invariant that provision/reject require principal_property + user_entity_type; (3) the exact unmatched predicate pinned.'
severity: significant
resolution: 'Addressed in this ticket''s design + implementation. The guard is derived from WIRING STATE (a.jwtGate != nil, passed into attachACLRequest as jwtVerified), NOT from a per-Principal marker — confirmed correct since JWT and header principals both stamp Tool=ToolDataEntry. The ambiguous-unmatched-predicate half is closed by the load invariant: reject/provision require both principal_property and user_entity_type (validateUnmatchedPrincipal in policy.go), so ''unmatched'' is only ever evaluated when the lookup is actually enabled — otherwise every request would look unmatched. The flag is set only on a genuine no-match (id=="") while the lookup is enabled. Pinned by TestReject_HeaderPrincipalUntouched, TestUnmatchedPrincipal_RejectIgnoresUnflagged (scheduler), and TestUnmatchedPrincipal_RejectRequiresLookup. The code reviewer independently verified header/CLI/scheduler are untouched.'
status: addressed
---

See title. Guard from wiring state (`a.jwtGate != nil`), not from an unavailable
per-Principal marker; add the load invariant `provision|reject ⇒
principal_property + user_entity_type set`; pin the exact unmatched predicate.
