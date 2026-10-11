---
id: RR-FBUQP1
type: review-response
title: restish behaviour claims in docs are unverified
finding: docs/restish.md assumes restish sends no cookie or Origin and picks the octet-stream body.
severity: minor
resolution: 'Verified end to end through Pratique: restish -v shows Content-Type application/octet-stream and the PUT succeeded with no Origin or cookie; the PNG round-tripped byte-identical. Recorded in IMPL-YCKA30.'
status: addressed
---
