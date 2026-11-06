---
id: RR-KZBZ5FG
type: review-response
title: '[security] Readers can exhaust room and peer slots'
finding: '[security] A read-only join takes a peer slot and can create a room, with no cap per principal. One reader can fill a room''s 32 slots or all 2048 rooms and lock others out of co-editing.'
severity: minor
resolution: Covered by the per-principal connection cap (16) and the global cap (4096).
status: addressed
---
