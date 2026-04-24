---
id: RR-NJ8JJ
type: review-response
title: Multi-value response headers truncated to first value
finding: Only first value of multi-value headers is kept. Set-Cookie would be lossy.
severity: significant
reason: Documented limitation. First-value-wins is sufficient for the API-calling use case this module targets (JSON/REST integrations where multi-valued response headers are rare and lossless round-tripping of Set-Cookie is out of scope). Scripts needing full header arrays can read the underlying response via http.request directly and parse canonicalized headers; a follow-up ticket can add an opt-in array-shaped headers table if a concrete use case appears.
status: wont-fix
---
