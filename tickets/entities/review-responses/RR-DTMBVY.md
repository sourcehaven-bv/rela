---
id: RR-DTMBVY
type: review-response
title: 401 has no WWW-Authenticate header
finding: RFC 9110 expects a challenge on 401; the JWT gate sends Bearer when its header is Authorization.
severity: minor
reason: Without a JWT gate there is no credential scheme to name. The writeNoIdentity comment now says so.
status: wont-fix
---
