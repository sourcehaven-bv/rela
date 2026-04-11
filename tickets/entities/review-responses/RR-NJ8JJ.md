---
id: RR-NJ8JJ
type: review-response
title: Multi-value response headers truncated to first value
finding: Only first value of multi-value headers is kept. Set-Cookie would be lossy.
severity: significant
reason: Documented limitation. First-value-wins is sufficient for the API-calling use case. Can be enhanced later if needed.
status: wont-fix
---
