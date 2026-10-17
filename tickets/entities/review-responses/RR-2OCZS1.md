---
id: RR-2OCZS1
type: review-response
title: Only post is rate limited; fs edit rewrites a segment under a global lock
finding: Edit, remove and create have no bucket; looping PATCH rewrites up to 16 MB per edit under channels.lock. Charge buckets per op and add a per-identity aggregate bucket.
severity: significant
resolution: 'Plan updated: .ignored/t3-plan.md section 6 (Design review folded in) redesigns this point; implementation follows that section.'
status: addressed
---
