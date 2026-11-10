---
id: RR-67UYXV7
type: review-response
title: 'Code review: watcher nits'
finding: onRefused must not call stop; the watch test did not prove a lookup ran; hard-coded 30 seconds in messages.
severity: nit
resolution: Documented the stop contract; the test counts lookups; messages use tenant.RevalidateInterval.
status: addressed
---
