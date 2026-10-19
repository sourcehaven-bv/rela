---
id: RR-M3PYE4
type: review-response
title: 'D1: a held leader lock is simpler than a lock per round'
finding: Every pg node already receives every event through NOTIFY. One process holding `lock.For(st)` or the fs filelock for its lifetime, and running the worker alone, removes duplicate reads, N reconcile scans, and the unlocked-scan/locked-re-read dance, while the shared KV still lets another node take over. Per-round locking pins a pool connection on every node every tick and buys nothing that phase 5 keeps.
severity: significant
resolution: 'Adopted per D1: one leader holds lock.For (pg) or the filelock (fs) for its lifetime; another process takes over when it is released.'
status: addressed
---
