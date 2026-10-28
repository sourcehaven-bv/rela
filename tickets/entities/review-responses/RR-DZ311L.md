---
id: RR-DZ311L
type: review-response
title: Blanket untested_denial reason did not fit three handlers
finding: handleSSE, handleV1Leases and handleV1Piles do refuse, and tests cover it.
severity: nit
resolution: Those three entries name their tests; the shared reason now says archplan does not link a Go test where a refusal exists.
status: addressed
---
