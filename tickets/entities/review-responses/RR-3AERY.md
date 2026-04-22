---
id: RR-3AERY
type: review-response
title: origin-security canary only covers GET requests
finding: The whole point of an Origin allowlist is to block cross-site writes. A GET-only canary misses the primary attack surface — a regression exempting POST/PATCH/DELETE would pass.
severity: critical
resolution: Added POST/PATCH/DELETE + mismatched-Origin tests to origin-security.spec.ts. 7 canary tests total.
status: addressed
---
