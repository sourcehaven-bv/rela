---
id: RR-HEBJFQ
type: review-response
title: HTTP failures written with the unprovisioned request
finding: fail.writeHTTP gets the original r, not the provisioned one.
severity: nit
resolution: Added a comment that no failure writer reads the principal.
status: addressed
---
