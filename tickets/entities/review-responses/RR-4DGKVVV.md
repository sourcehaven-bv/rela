---
id: RR-4DGKVVV
type: review-response
title: Fork test helper uses want 0 as a mode flag
finding: settledGoroutineCount switches behavior on want == 0; two functions would be clearer.
severity: nit
reason: Test-only helper in the fork; the behavior is documented in its comment. Can be tidied with TKT-TD68KKT.
status: wont-fix
---
