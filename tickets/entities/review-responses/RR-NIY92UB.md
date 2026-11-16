---
id: RR-NIY92UB
type: review-response
title: Postgres test does not use the shared DSN helper
finding: The postgres differential opened its own connection instead of the backendtest helper.
severity: minor
resolution: It now uses backendtest.DSN, pgstore.NewPool and pgstore.Open.
status: addressed
---
