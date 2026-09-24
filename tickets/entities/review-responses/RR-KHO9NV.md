---
id: RR-KHO9NV
type: review-response
title: Reconcile locking and PRAGMA optimize
finding: writeMu unnecessary; PRAGMA optimize conflicts with the DSN-only PRAGMA rule and changes all plans.
severity: minor
resolution: Reconcile runs in one BEGIN IMMEDIATE without writeMu; no PRAGMA optimize unless an EXPLAIN test proves it is needed.
status: addressed
---
