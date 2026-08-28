---
id: RR-JZ7VDI
type: review-response
title: Deleted-endpoint history + spoofed fromType fail-closed untested
finding: For a deleted-endpoint relation, authorizeRelationHistoryRead does not validate fromType against a live row, so serveRelationHistoryVersion synthesizes entityPkg.New(snap.From, fromType) from a caller-supplied, unvalidated URL segment. It is correct today (the type-level closed-world in RelationFieldVerdicts keys on relType not from.Type, so it fires regardless, and a bogus from.Type matches no grant → all hidden) but load-bearing and non-obvious, and no test exercised the deleted-endpoint + spoofed-fromType case.
severity: significant
resolution: 'Added TestRelationHistoryRedaction_DeletedEndpoint_FailsClosedDespiteFromType: drives the deleted-endpoint history path (GONE-A/GONE-B) with fromType ''bogus_type'' that matches no visible:-declaring type, holding history:read but NOT history:read-redacted, and asserts the meta (reason) is still hidden.'
status: addressed
---
