---
id: RR-BY3P
type: review-response
title: rebuildSearchIndex publishes empty index if IndexBatch fails
finding: If search.NewIndex() succeeds but idx.IndexBatch(docs) fails on line 434, the refactor still publishes the empty new index and closes the old one. That is a regression AND a pre-existing bug worth fixing while here. On IndexBatch error the new index should be closed and the old one left in place.
severity: significant
resolution: 'Extracted the rebuild into `buildReloadSearchIndex` helper. If `search.NewIndex()` fails, returns the old index from oldState (or nil if there wasn''t one). If `IndexBatch` fails, closes the partial candidate index AND returns the old index — we never publish an empty or partial index on top of a working one. Same pattern applied to `newWorkspace` construction: if IndexBatch fails during construction, the partial index is closed and no search is published.'
status: addressed
---
