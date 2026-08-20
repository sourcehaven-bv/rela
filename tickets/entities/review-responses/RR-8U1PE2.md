---
id: RR-8U1PE2
type: review-response
title: Per-state events would corrupt the default-world search index in Step 1
finding: Search indexers key documents by bare entity id (linearsearch.go:27 `l.entities[e.ID] = e.Clone()`; the bleve observer alike). With PR-A emitting store events per state, a write to (PAGE-1, draft) would OVERWRITE PAGE-1's default face in the index — draft content leaking into (and replacing) default-world search results. Per-world indexing is Step 5 (TKT-9KZGJO) / backfill TKT-9OJ3S0; Step 1 has no world machinery to index states correctly.
severity: significant
resolution: 'Plan updated (PR-A work list): the indexing observers SKIP events with a non-zero Pointer until Step 5 — the index remains exactly today''s default world, fail-safe. Pinned with a test in PR-A. Does not reopen the per-state-events decision (events still fire; the observer chooses).'
status: addressed
---
