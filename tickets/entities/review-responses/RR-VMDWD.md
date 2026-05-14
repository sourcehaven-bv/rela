---
id: RR-VMDWD
type: review-response
title: stale-review.lua audit — read the three update_entity call sites
finding: 'Risk #2 says ''in-repo scripts don''t validate-by-error''. Verified: tickets/scripts/stale-review.lua calls rela.update_entity at lines 187, 195, 207. Need to read whether they''re wrapped in pcall or treat errors as ''skip this entity''. If pcall-wrapped: post-ticket they''ll silently process entities that previously errored — behavior shift to verify acceptable. Recommendation: 3-min read, document conclusion in plan, add test if needed. From design-review F13.'
severity: minor
resolution: 'Audit completed. Read tickets/scripts/stale-review.lua lines 187, 195, 207. All three calls to rela.update_entity are NOT pcall-wrapped and ignore return values. Post-ticket: continues to work identically (entity table is first return; second nil warnings is silently dropped). Documented in Research section.'
status: addressed
---
