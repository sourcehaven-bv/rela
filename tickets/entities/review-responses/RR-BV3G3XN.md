---
id: RR-BV3G3XN
type: review-response
title: "Non-GET /_tenant answers 405 on a multi-tenant host"
finding: "The tenant host only intercepts GET, so other methods reach the tenant router, which now answers 405 instead of 404."
severity: minor
resolution: "Addressed: intended, since the route exists; the 405 carries Allow: GET and the test pins it."
status: addressed
---
