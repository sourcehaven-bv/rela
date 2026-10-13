---
id: RR-L71X0C
type: review-response
title: Doc understates per-call cost on fsstore/memstore
finding: On fs/mem each call scans the in-memory index, so a step reading per entity costs entities times relations.
severity: significant
resolution: 'Docs now state the cost per backend. A per-step cache was not added: results would have to be cached as Go values and re-converted per call to keep scripts from mutating shared tables, and pg/sqlite already use indexes; the motivating tenant is on pg.'
status: addressed
---
