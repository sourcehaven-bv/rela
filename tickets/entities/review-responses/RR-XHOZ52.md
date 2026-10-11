---
id: RR-XHOZ52
type: review-response
title: Tests do not prove Start runs once
finding: Two goroutines would still produce one version; the assertion passes either way.
severity: minor
resolution: Lifecycle tests count run-loop goroutines; two Starts add one and Start after Stop adds none; fails with the guard removed.
status: addressed
---
