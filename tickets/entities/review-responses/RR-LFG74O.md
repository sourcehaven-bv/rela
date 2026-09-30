---
id: RR-LFG74O
type: review-response
title: Guard RelationKey literals without FromFace
finding: A shrink-only allowlist test, like bareref_test.go for Ref, would flag new tail-less RelationKey literals in production code.
severity: nit
resolution: internal/archguard/tailless_test.go pins tail-less RelationKey literals in non-test code to a shrink-only allowlist with a reason per file (acl storegraph, sync push and splice).
status: addressed
---
