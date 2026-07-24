---
id: RR-MORL7M
type: review-response
title: Flushed versions must never carry rename markers (PrevID/PrevFrom/PrevTo empty)
finding: 'The lineage walk fences on rename rows and purge refuses to purge them. A flush is content-attribution only: it must always use op ∈ {create, update} with PrevID (entity) / PrevFrom, PrevTo (relation) empty, so a flushed row is never mistaken for a rename marker by the lineage CTE or purge''s rename-refusal.'
severity: minor
reason: Flush split into follow-up TKT-0IGI4V; pinned there as 'Never a rename marker'.
status: deferred
---
