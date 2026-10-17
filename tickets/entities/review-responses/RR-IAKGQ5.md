---
id: RR-IAKGQ5
type: review-response
title: rekey compares stale token counts
finding: rekey compares raw tokens taken at different times instead of tokensAt(now).
severity: nit
reason: 'Effect is negligible: both buckets refill toward full and only the emptier is kept on a rare rename clash.'
status: wont-fix
---
