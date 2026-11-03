---
id: RR-PLD7ZY3
type: review-response
title: Old instance flushed in both the route watcher and autoSaveFor
finding: Both places call commitImmediately on the outgoing instance.
severity: nit
reason: The watcher flushes before the fetch starts; autoSaveFor covers any load that replaces the row without the watcher. commitImmediately is idempotent, so the second call is free.
status: wont-fix
---
