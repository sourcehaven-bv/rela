---
id: TKT-ZZL53L
type: ticket
title: Drop or use entities_search_tsv_idx
kind: chore
priority: low
effort: xs
status: wont-fix
---

Design doc §12.8. The index is created but never queried — no `to_tsvector`/`@@`
in any Go file. A maintained GIN index with no read path is pure write
amplification. Drop it, or wire the tsvector query path.

## Resolution

Closed: Done: pgstore migration 0014_read_indexes.sql drops
entities_search_tsv_idx. Status is wont-fix, not done, because this ticket has
no review checklist.
