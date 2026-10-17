---
id: RR-U23CH8
type: review-response
title: Reconciler detaches channels on a temporary file absence
finding: A git checkout or stash removes an entity file past the watcher debounce. The reconciler tombstones its channels and discards the handle; when the file returns nothing restores them.
severity: significant
resolution: 'Reconciler removed: an external delete no longer detaches channels, so a temporary file absence (git checkout) leaves them intact and an external re-create gets them back.'
status: addressed
---
