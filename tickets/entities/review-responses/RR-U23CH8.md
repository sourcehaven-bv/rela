---
id: RR-U23CH8
type: review-response
title: Reconciler detaches channels on a temporary file absence
finding: A git checkout or stash removes an entity file past the watcher debounce. The reconciler tombstones its channels and discards the handle; when the file returns nothing restores them.
severity: significant
resolution: The reconciler keeps external detaches in a bounded ledger (cap 1024) and restores them on an external EventEntityCreated; occupied anchors are logged and dropped. Tested by TestChannelReconciler_RestoresOnExternalCreate and TestExternalDetaches_Bounded.
status: addressed
---
