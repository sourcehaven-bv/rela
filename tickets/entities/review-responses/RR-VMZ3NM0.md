---
id: RR-VMZ3NM0
type: review-response
title: 'Design: fingerprint ops escape the row budget'
finding: 'A fingerprint over a type with a visible: restriction or roll-up takes the list path and loads up to maxRows rows, so 32 fingerprint ranges cost about 64000 row loads while only list limits were budgeted.'
severity: critical
resolution: 'The work bound is slot time: a request is admitted to one of a few server-wide slots (and at most two per principal) and runs under a deadline; ranges it does not reach answer not_run. Elements runs one range at a time so memory stays at one page. Charging fingerprints maxRows each would cap a round at a few ranges, and the handler cannot tell the fold path from the list path without the scope internals.'
status: addressed
---
