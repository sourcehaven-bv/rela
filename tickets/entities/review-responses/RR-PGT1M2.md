---
id: RR-PGT1M2
type: review-response
title: Incoming hop with empty From; symmetric check after either lookup
finding: An empty From means the landing type of an incoming hop is unknown and must be refused. The symmetric check must apply whichever name matched.
severity: minor
resolution: 'resolveHopRelation applies the symmetric refusal whichever name matched, and an incoming hop over a relation whose From is empty is refused. Test: TestResolveTraversal_IncomingOverEmptyFromIsRefused.'
status: addressed
---
