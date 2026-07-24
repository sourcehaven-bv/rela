---
id: RR-SRZK6X
type: review-response
title: Reader.Get authorizes against caller-supplied type — cross-type read escalation (BUG-ZWTDH9 analog)
finding: 'The plan proposed that Get ''gates on the CALLER''s type claim'' and mirrors dataentry getVisible by leaving the stored-type check to consumers. Verified in code: visibleReader.getVisible never checks e.Type == entityType after load — today each dataentry caller re-checks. In a SHARED package that per-consumer convention is exactly the bug class of BUG-ZWTDH9 (authorize against caller-supplied type, act on stored type): PermitsRead(publicType, id) can permit under the public type''s rules, then the load returns an entity of a DENIED type and the Reader hands it out redacted under the wrong type''s field policy. Get must verify stored type == claimed type post-load and return (nil,false,nil) on mismatch, inside the package.'
severity: critical
resolution: 'Plan revised: the stored-type check (e.Type == entityType, else indistinguishable not-found) is now part of PolicyReader.Get''s in-package contract (PLAN-RR12W4 AC2), with a dedicated conformance case (allowed-type claim over a denied-type entity → miss). PR 2 will remove the now-redundant caller-side checks instead of double-checking.'
status: addressed
---
