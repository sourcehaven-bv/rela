---
id: RR-WXZ00T
type: review-response
title: 'Incoming query cannot use From: Direction gates only EntityID, so the obvious spelling silently returns outgoing edges'
finding: 'store.RelationQuery.Direction is honored for EntityID/EntityIDs only. From and To are matched unconditionally and independently (internal/store/storeutil/storeutil.go:327-348; endpointMatches at :354), and there is no ValidateRelationQuery to reject a contradictory query. Today''s query (internal/lua/deps.go:113-117) passes From: fromID AND Direction: DirectionOutgoing, where the Direction is redundant — From alone does the work. So the minimal-looking edit an implementer makes (keep From, flip Direction to Incoming) returns the entity''s OUTGOING edges. It compiles, runs, produces plausible numbers, and nothing rejects it. The plan named the wrong-endpoint risk but its mitigation — put the far-end choice inside the adapter — guards only the second half of a two-step mistake: picking rel.From vs rel.To correctly is worthless if the query returned the wrong edge set.'
severity: critical
resolution: 'Verified empirically against storeutil.NewRelationMatcher with one edge A->B: {From:''B'',Incoming} matches nothing; {From:''A'',Incoming} still matches A->B (an outgoing edge); {EntityID:''B'',Incoming} matches correctly. Recorded as a hard CONSTRAINT in both ticket and plan: incoming must be spelled {EntityID: id, Direction: DirectionIncoming} or {To: id}, never {From: id, Direction: ...}. The direction test must assert the returned edge SET, not merely which endpoint was read from it — a test that only swaps endpoint-picking would still pass against the wrong query.'
status: addressed
---

Empirically confirmed with a throwaway test against
`storeutil.NewRelationMatcher`; all three predicted behaviours reproduced
exactly.
