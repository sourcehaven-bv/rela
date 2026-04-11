---
id: RR-H4W3L
type: review-response
title: Convenience opts silently ignore invalid types
finding: parseConvenienceOpts silently ignores invalid types for headers and timeout, inconsistent with parseHTTPRequestOpts which raises.
severity: significant
resolution: parseConvenienceOpts now raises on type mismatches, consistent with request()
status: addressed
---
