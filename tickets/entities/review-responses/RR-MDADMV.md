---
id: RR-MDADMV
type: review-response
title: Full-ring Retry-After uses the oldest insert
finding: ring[0] is not the first to expire when types have different TTLs.
severity: minor
resolution: Retry-After uses the earliest expiry in the ring.
status: addressed
---
