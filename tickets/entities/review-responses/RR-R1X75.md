---
id: RR-R1X75
type: review-response
title: New http.Client per request, no default timeout
finding: Each request allocates a new http.Client with no timeout. No connection pooling, and requests without explicit timeout can hang forever.
severity: critical
resolution: Shared package-level httpClient with 30s default timeout and connection pooling
status: addressed
---
