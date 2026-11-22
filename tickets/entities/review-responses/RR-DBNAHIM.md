---
id: RR-DBNAHIM
type: review-response
title: Schema lookup error cached
finding: pgstore caches a schema lookup error from a cancelled ctx and disables instant reads until restart.
severity: significant
resolution: instantDB resolves the schema under a mutex and caches it only after success, so a failed lookup is retried on the next read.
status: addressed
---
