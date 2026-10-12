---
id: RR-U23CH8
type: review-response
title: Reconciler detaches channels on a temporary file absence
finding: A git checkout or stash removes an entity file past the watcher debounce. The reconciler tombstones its channels and discards the handle; when the file returns nothing restores them.
severity: significant
status: open
---
