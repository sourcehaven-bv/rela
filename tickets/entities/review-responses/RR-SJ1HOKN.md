---
id: RR-SJ1HOKN
type: review-response
title: No per-run byte budget
finding: A hostile source could push max_fetch rows of 4 MiB each per run.
severity: minor
resolution: 'New max_bytes (default 256 MiB) caps the bytes fetched per run; rows over it wait. TestRun_ByteBudget. A create budget is not added: every entity create needs a fetch, so max_fetch and max_bytes bound creates.'
status: addressed
---
