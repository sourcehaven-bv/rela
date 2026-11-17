---
id: RR-QGW74W9
type: review-response
title: Port probe can hang
finding: http.Get on the default client has no timeout.
severity: minor
resolution: 'Addressed: a net.Dialer with a 2 s timeout and the run context.'
status: addressed
---
