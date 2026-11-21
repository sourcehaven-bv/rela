---
id: RR-SMBZHZM
type: review-response
title: Unauthenticated callers reach token verification
finding: 'Security review: anyone can send a bearer token to the API path, which makes the verifier parse it and may refresh the JWKS for an unknown kid.'
severity: minor
resolution: 'Kept: the identity verifier bounds the unknown-kid refresh rate and the refresh interval, as for assertions and webhooks. Rejections are logged at info without the token.'
status: addressed
---
