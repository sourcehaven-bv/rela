---
id: RR-L1BWRO
type: review-response
title: ready id on resume skips replayed changes
finding: If ready carries the head token on resume, a client that drops before the first flush stores a position past the replayed changes and loses them.
severity: critical
resolution: On resume ready carries the presented position (sub.Start()); only a fresh connect or resync starts at head. AC3 test covers reconnecting with the ready token before the flush.
status: addressed
---
