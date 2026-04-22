---
id: RR-I5WME
type: review-response
title: rela.cache namespace collision when one script serves multiple docs
finding: 'rela.cache is namespaced per scriptPath (cache.go:276). Two data-entry.yaml document entries pointing at the same .lua file share a cache namespace even though their ConfigIDs differ. Author-error waiting to happen: rela.cache.memoize(''summary:'' .. entry_id, ...) collides across docs.'
severity: significant
resolution: 'After discussion with user: cache stays namespaced per script path (keeping shared helper scripts useful). Caveat documented in GUIDE-data-entry (AC-DOC1); prototype example demonstrates explicit doc-scoped keys via rela.document.id.'
status: addressed
---

From design-review on PLAN-78HJO.

Two options: (a) Document the caveat; tell script authors to include
`rela.document.id` in cache keys. (b) In document mode, namespace cache by
`scriptPath + "|" + documentID` so default is safe.

(b) is stronger; (a) is less work. Given the ticket is already about adding the
feature, (b) seems right — preserve the namespace-by-script-path default
elsewhere, special-case document mode.
