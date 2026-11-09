---
id: RR-P5HOOIS
type: review-response
title: 'Design: mid-request failure semantics undefined'
finding: A failure at range 17 after earlier results were written leaves a partial 200.
severity: significant
resolution: The response is buffered and written once. Per-range status is ok, too_large or not_run (deadline). Any other error fails the whole request with the mapped status and no partial body.
status: addressed
---
