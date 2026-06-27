---
id: RR-X56H
type: review-response
title: TKT-VMD8 DenyAll short-circuit ordering unpinned — leaks search-backend timing signal
finding: 'AC4 says ''no-read principal hitting /api/v1/tickets gets data:[]'' but the scope bullet for scopedSortedEntities just says ''DenyAll → return empty slice.'' Allows two implementations: (a) return nil before freeTextIDsForType runs, or (b) run search/filter/sort against empty slice. Both produce same wire output but (b) wastes a Bleve query AND leaks a constant-time signal that the search backend was hit. Fix: pin in AC4 ''DenyAll short-circuits before freeTextIDsForType, applyV1Filters, and applyV1Sorting; the search backend MUST NOT be invoked.'' Regression test with a mock searcher that fails the test if invoked on a DenyAll path.'
severity: critical
reason: 'AC4 DenyAll short-circuit ordering is a TKT-VMD8 acceptance criterion targeting scopedSortedEntities — the function that takes a list of entities and runs free-text search + filters + sort. This PR does not touch scopedSortedEntities, freeTextIDsForType, applyV1Filters, or applyV1Sorting. There is no list-side DenyAll path in this PR to pin ordering on; the per-entity gate this PR adds is a 1-id check, not a list pipeline. The critical severity reflects "if uncorrected in TKT-VMD8, the search backend is invoked even under DenyAll and a constant-time signal leaks" — that''s a real concern, but it''s a TKT-VMD8 merge blocker, not a TKT-VQGN merge blocker. Deferring keeps the gate-vs-list separation the original PR split set up.'
status: deferred
---
