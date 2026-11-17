---
id: RR-WYC7Z81
type: review-response
title: Background job dropped the client ceiling
finding: The auto-apply job stamped only the user, so a scoped or client principal ran with the user's full grants.
severity: significant
resolution: The payload carries principal type and scopes; the handler restamps with principal.VerifiedFrom and the same claims. TestLLMActionJob_CeilingAndRedelivery pins it.
status: addressed
---
