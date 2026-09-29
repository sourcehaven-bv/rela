---
id: RR-LFG74O
type: review-response
title: Guard RelationKey literals without FromFace
finding: A shrink-only allowlist test, like bareref_test.go for Ref, would flag new tail-less RelationKey literals in production code.
severity: nit
reason: Useful follow-up; the remaining literals are commented and deliberate. Not required for the flip.
status: deferred
---
