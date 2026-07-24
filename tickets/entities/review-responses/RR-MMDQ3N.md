---
id: RR-MMDQ3N
type: review-response
title: Flush op (create vs update) must reuse the sweep's delete-fenced lineage probe
finding: An entity created but never yet swept, then edited by a second author, has NO version in its lineage — the flushed pre-edit snapshot must be op=create (else history starts with an update). A re-created-after-delete entity must also fence correctly. The flush must reuse the sweep's selectCandidates-equivalent lvc/live-lineage logic (two-LATERAL delete fencing) for BOTH the dedup hash AND the op choice, so a flushed version is indistinguishable from a swept one.
severity: significant
reason: 'Constrains the flush''s create-vs-update op choice; the flush mechanism itself was split to follow-up TKT-0IGI4V per RR-K781MZ, so this PR contains no code path that chooses a version op outside the sweep''s existing delete-fenced probe. The requirement is recorded verbatim in TKT-0IGI4V (''Op choice'': reuse the sweep''s two-LATERAL lvc/live-lineage logic for both dedup hash and op) so a flushed version will be indistinguishable from a swept one when that ticket is implemented.'
status: deferred
---
