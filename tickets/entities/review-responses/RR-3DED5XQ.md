---
id: RR-3DED5XQ
type: review-response
title: Cancelled client is logged as 200
finding: Returning without a write records an implicit 200.
severity: minor
resolution: The handler writes 499 with no body. Pinned by TestReconcile_ClientCancelIsNotNotRun.
status: addressed
---
