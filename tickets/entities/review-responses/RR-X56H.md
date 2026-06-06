---
id: RR-X56H
type: review-response
title: TKT-VMD8 DenyAll short-circuit ordering unpinned — leaks search-backend timing signal
finding: 'AC4 says ''no-read principal hitting /api/v1/tickets gets data:[]'' but the scope bullet for scopedSortedEntities just says ''DenyAll → return empty slice.'' Allows two implementations: (a) return nil before freeTextIDsForType runs, or (b) run search/filter/sort against empty slice. Both produce same wire output but (b) wastes a Bleve query AND leaks a constant-time signal that the search backend was hit. Fix: pin in AC4 ''DenyAll short-circuits before freeTextIDsForType, applyV1Filters, and applyV1Sorting; the search backend MUST NOT be invoked.'' Regression test with a mock searcher that fails the test if invoked on a DenyAll path.'
severity: critical
status: open
---
