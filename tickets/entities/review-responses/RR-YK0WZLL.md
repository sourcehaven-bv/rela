---
id: RR-YK0WZLL
type: review-response
title: 'Design: client could send the key over plain HTTP or follow a redirect'
finding: A peer key does not expire, so the client must not leak it over http or to a redirect target.
severity: significant
resolution: 'Already true in httpremote: plain http is refused for a non-loopback host unless --allow-insecure-http, CheckRedirect returns ErrUseLastResponse, and the Authenticator redacts itself in String/GoString. A test covers --token-secret over the same path.'
status: addressed
---
