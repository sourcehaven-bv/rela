---
id: RR-WBGKG1P
type: review-response
title: Fake cannot produce empty 412 or applied-then-lost writes
finding: The fake checks and writes in one step; add faults for a 412 with empty conflicts and for a write applied then answered 503.
severity: significant
resolution: 'Added faults: empty412 (412 naming no field), exhaust (three empty 412s, every attempt of one save, with a floor), and lost (applied, then answered 503).'
status: addressed
---
