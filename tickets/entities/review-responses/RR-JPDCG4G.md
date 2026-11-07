---
id: RR-JPDCG4G
type: review-response
title: Action error without asserted_roles acknowledged with 200
finding: A webhook action returning message_type error without asserted_roles ended in a 200 while the docs promised a 502 and a forgotten jti. The IdP did not retry so a revocation could be lost silently (CISO review on PR 2113).
severity: significant
resolution: dispatchWebhookAction returns errWebhookActionError for any message_type error response before roles are touched so the receiver answers 502 and forgets the jti. Pinned by the error cases in TestWebhook_StoresAssertedRoles including a retry of the same jti. Docs and examples/idp-sync.lua updated.
status: addressed
---
