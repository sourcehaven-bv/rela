---
id: RR-XA3R
type: review-response
title: Close + concurrent Reload leaks the search index and allows resurrection
finding: 'Close says ''must not run concurrently with Reload'' but nothing enforces it. If a watcher-triggered Reload is in flight when Close is called, the Reload can publish a new index on top of nil, leaking the Bleve index handles. Also: a Reload arriving after Close happily re-creates automation engine and search index, resurrecting a closed workspace. No ''closed'' flag.'
severity: critical
resolution: 'Fixed with three changes: (1) added `closed atomic.Bool` to Workspace; (2) Reload checks `w.closed.Load()` at the top under reloadMu and returns an error if the workspace is already closed; (3) Close takes `reloadMu` so it serializes against any in-flight Reload, and uses `w.closed.Swap(true)` to make itself idempotent and to transition into the closed state atomically. After Close, any Reload that arrives sees closed=true and returns an error instead of resurrecting the workspace.'
status: addressed
---
