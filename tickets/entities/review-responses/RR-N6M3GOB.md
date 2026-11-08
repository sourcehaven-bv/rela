---
id: RR-N6M3GOB
type: review-response
title: 'Code: unbound ctx succeeds when no entity type is named'
finding: Only ReadQueryFor checked the bound request so a call with no entity types answered an empty range instead of failing closed.
severity: minor
resolution: compile calls ElementReads.Bind first; AllowAllReadQueries gained Bind. TestElements_FailsClosedWithoutRequest covers no types and relations only.
status: addressed
---
