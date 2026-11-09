---
id: RR-JMX0MV2
type: review-response
title: 'Gate: a 503 refusal spent a token'
finding: AllowN took the token before the per-principal and slot checks, and the busy refusals did not return it, so a peer retrying on 503 drained its burst.
severity: significant
resolution: acquire reserves one token, cancels it on 429 and on both busy refusals. Pinned by TestReconcileGate/a busy refusal spends no token.
status: addressed
---
