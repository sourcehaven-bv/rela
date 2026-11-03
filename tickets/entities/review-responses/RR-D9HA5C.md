---
id: RR-D9HA5C
type: review-response
title: Timer and gate must share one expiry rule
finding: VerifyAssertion sets no jwt.WithLeeway. Today the gate rejects a token at exp by the server clock, and the timer fires at exp by the same clock, so the two agree. Someone may later add leeway for IdP clock skew, which is a common change. The gate would then accept tokens up to exp+leeway, while the timer fires at exp, or at once if exp has passed. Every stream opened in that window would open and close at once. Because onopen resets the SPA backoff, the client would reconnect every 2 s until the window ends. Make the coupling explicit. Document on AssertionClaims.ExpiresAt that it is the moment the gate stops accepting the token, so any future leeway is added to ExpiresAt in the verifier as well as to the parser options. Add a comment at the timer that points to it.
severity: minor
resolution: 'Plan updated: AssertionClaims.ExpiresAt is defined as the moment the verifier stops accepting the token (exp plus any leeway).'
status: addressed
---
