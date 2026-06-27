---
id: RR-KPPR
type: review-response
title: _position?id=... is a per-id chokepoint that PR 1 must gate (contradicts ticket framing)
finding: 'TKT-VQGN''s framing is ''anything that targets a single, known entity ID returns 404 if the principal can''t see it.'' But handleV1EntityPosition (scope.go:169) reads `id` from URL, resolves the full scope, then searches for the id linearly. Under PR 1 the scope list is unfiltered (list ACL lands in PR 2), so a hidden entity is in the list — the position call returns 200 with current:N, total:M, leaking BOTH existence AND ordinal. Per-id oracle today. Either gate _position in PR 1 (consistent with the framing; small change: at the top of handleV1EntityPosition after bad-id guards, getEntity the id to learn its type, then probe Visible and return the existing not_in_scope 404 on deny) OR rewrite the ticket framing to explicitly exclude _position. Recommend gate in PR 1.'
severity: critical
reason: 'Deferred per user direction: _position is list-derived (computes from a scope walk), grouped with other list-style endpoints in a follow-up ticket after PR 2 lands. TKT-VQGN framing tightened to ''per-entity-response'' (GET / writes / include) to make this scoping explicit.'
status: deferred
---
