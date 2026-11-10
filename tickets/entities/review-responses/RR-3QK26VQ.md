---
id: RR-3QK26VQ
type: review-response
title: In-memory run store is per process
finding: Without a cache directory the run store is in-memory, so the scheduler and the server do not see each other's runs and the ceiling differs between them.
severity: minor
resolution: Documented in GUIDE-flows.
status: addressed
---
