---
id: RR-MVX6N4J
type: review-response
title: No per-identity file post rate
finding: FileRate is per channel, so one identity could spread file posts over many channels.
severity: minor
resolution: Post also takes FileIdentityRate (30 per minute) on the identity bucket.
status: addressed
---
