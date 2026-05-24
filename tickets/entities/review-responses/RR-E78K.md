---
id: RR-E78K
type: review-response
title: Search subtest passes when L1264 (Searcher.Search) is reverted — shared recorder hides the second site
finding: 'Empirically confirmed: revert ONLY line 1264 (Searcher.Search) back to context.Background() and the TestReadBindings_PropagateCallerContext/search subtest still passes. Root cause: a single shared *ctxRecorder is overwritten on every record() call. In the search path: (1) ctxSpySearcher.Search records (broken case: nil); (2) mockSearcher iterates and yields hits; (3) per-hit ctxSpyStore.GetEntity records (correct: ''parent-marker''). Last write wins. The test only verifies one of the two L1264/L1270 sites.'
severity: critical
resolution: 'Refactored test to use Option C: ctxRecorder now stores a slice of ctxCall {method, marker, hasMarker} entries. Each spy method records with its own method name. Tests assert EVERY recorded call carries the parent marker — so reverting either L1264 (Searcher.Search) or L1270 (Store.GetEntity) independently produces a distinct, targeted failure. Verified both reverts now fail the search subtest with named call-site messages.'
status: addressed
---
