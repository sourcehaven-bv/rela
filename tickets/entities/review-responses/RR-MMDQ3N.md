---
id: RR-MMDQ3N
type: review-response
title: Flush op (create vs update) must reuse the sweep's delete-fenced lineage probe
finding: An entity created but never yet swept, then edited by a second author, has NO version in its lineage — the flushed pre-edit snapshot must be op=create (else history starts with an update). A re-created-after-delete entity must also fence correctly. The flush must reuse the sweep's selectCandidates-equivalent lvc/live-lineage logic (two-LATERAL delete fencing) for BOTH the dedup hash AND the op choice, so a flushed version is indistinguishable from a swept one.
severity: significant
reason: Flush split into follow-up TKT-0IGI4V; pinned there as 'Op choice' (reuse the sweep's delete-fenced lineage probe).
status: deferred
---
