---
id: RR-FV3O0H
type: review-response
title: statsResponseWriter does not forward http.Hijacker
finding: A direct w.(http.Hijacker) assertion would fail under Debug and succeed without it; no handler hijacks today.
severity: minor
resolution: 'Documented on the type: add a forwarder before any handler hijacks. Not forwarded speculatively — an unused interface method is the kind of API that rots.'
status: addressed
---
