---
id: RR-7T26FZ
type: review-response
title: Side table needs bookkeeping on every row-moving path
finding: A side table must be maintained on create, update, both deletes, rename, mark, unmark and PurgeMarked on two backends.
severity: significant
resolution: 'pg: mention_targets text[] column on entities with a GIN index, written in the same statement as search_text; every row move carries it. sqlite: JSON column plus a trigger-maintained fan-out table via json_each, mirroring entity_search.'
status: addressed
---
