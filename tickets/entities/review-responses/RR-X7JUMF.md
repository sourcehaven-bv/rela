---
id: RR-X7JUMF
type: review-response
title: 'Eager far-entity resolution: bare min/max gates now read one entity per edge where they previously read none'
finding: 'Code review. The pre-seam code called GetEntity only inside `if len(whereFilters) > 0` (origin/develop internal/validation/validation.go:591). The new adapter resolves every edge in RelatedEntities before the evaluator can decline, so a constraint with no `where` and no `target_type` now performs one entity read per edge. Two of the 14 shipped gates are bare `min: 1` (planning-ticket-needs-checklist, analyzing-bug-needs-checklist). Measured: 5 gets for a constraint that previously did 0. Invisible on tickets/ and serious on a postgres deployment running analyze over a large graph — it is a per-row lookup on a collection read, the defect CLAUDE.md''s collection-reads rule exists to prevent. The ticket''s own Cost section said this change must not make cost worse.'
severity: significant
resolution: validation.Graph.RelatedEntities gained a resolveFar bool; the evaluator computes it as `len(whereFilters) > 0 || c.TargetType != ""` and the adapter skips GetEntity entirely when false, returning elements with just the far ID. Pinned by TestRelatedEntities_NoFarReadsWhenNotRequested, which uses a counting reader to assert 0 reads when not requested and 2 when requested — a budget test rather than a behavioural one, since the count was always correct and that is precisely why no existing test caught it.
status: addressed
---

My own plan flagged "a performance change must not ride along with a semantics
change" and then shipped the opposite: a silent performance regression inside a
semantics change. The behavioural tests could not catch it because the counts
stayed right.
