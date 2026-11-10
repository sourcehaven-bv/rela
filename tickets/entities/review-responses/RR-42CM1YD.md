---
id: RR-42CM1YD
type: review-response
title: 'Design: webhook memo rests on a false replay-set premise; events without org_id cannot be routed'
finding: jwtauth.WebhookVerifier keeps no replay set, so ReuseWebhook is unnecessary. Events outside membership/organization need no org_id and so cannot be routed to a tenant; the host must decide explicitly and not drop them silently.
severity: significant
resolution: 'No webhook memo: the jwtauth webhook verifier keeps no replay set so the host verifies to route and the tenant receiver verifies again. No org_id answers 400 with a Warn log; unknown org is acknowledged. Pinned by TestHost_Webhook.'
status: addressed
---
