---
id: RR-X0V93Q
type: review-response
title: CalDAV took a transient error for a hidden row
finding: entityIsHidden used the gated content read; it turns a store error into not-found; so a PUT during an outage got the permanent 404 and the client dropped its copy
severity: critical
resolution: entityIsHidden now decides from loadStoredFaces and PermitsReadFace only; any error keeps the retryable 503
status: addressed
---
