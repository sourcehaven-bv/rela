---
id: RR-94DB8Q
type: review-response
title: Naive fallback is silent
finding: A query that falls back to graphquerynaive leaves no trace.
severity: nit
reason: The fallback is correct and only triggers on property names containing a quote, backslash or control character; the harness keeps it covered.
status: wont-fix
---
