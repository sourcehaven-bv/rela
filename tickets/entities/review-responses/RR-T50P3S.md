---
id: RR-T50P3S
type: review-response
title: Link header on every method and status of /
finding: withSpecLink added the header on POST and other methods too.
severity: nit
resolution: Restricted to GET and HEAD; the unit test covers POST without the header.
status: addressed
---
