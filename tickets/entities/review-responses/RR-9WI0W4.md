---
id: RR-9WI0W4
type: review-response
title: Failed local-role batch was silent
finding: PrimeLocalRoles returned ctx unchanged on a store error with no log, so a broken batch would only show up as slow requests.
severity: minor
resolution: It now logs a Warn with the entity count and error before falling back to the per-entity walk.
status: addressed
---
