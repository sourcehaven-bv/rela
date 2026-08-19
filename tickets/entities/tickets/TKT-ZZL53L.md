---
id: TKT-ZZL53L
type: ticket
title: Drop or use entities_search_tsv_idx
kind: chore
priority: low
effort: xs
status: backlog
---

Design doc §12.8. The index is created but never queried — no `to_tsvector`/`@@`
in any Go file. A maintained GIN index with no read path is pure write
amplification. Drop it, or wire the tsvector query path.
