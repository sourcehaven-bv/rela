---
id: FEAT-ARR07
type: feature
title: 'Store write transactions: Tx on store.Store'
description: 'A write-transaction contract on store.Store (DEC-8UIL0): Tx(ctx, fn) serializes the callback against all other writers with per-backend mechanics — postgres gets a native transaction + deployment-wide advisory lock with rollback and post-commit events; fs/memstore get a write mutex with reduced single-user guarantees. Foundation for entitymanager intent atomicity, the Lua rela.tx helper, and the eventual deletion of dataentry writeMu.'
status: in-progress
---

Per-backend write-transaction contract on `store.Store` (DEC-8UIL0): `Tx(ctx, fn
func(Store) error)` serializes the callback against all other writers —
cross-process on postgres (native transaction + advisory lock, rollback on
error, events at commit), in-process on fs/memstore (write mutex; reduced
guarantees for single-user deployments: no rollback, inline events). Foundation
for entitymanager intent atomicity and the eventual deletion of dataentry
writeMu.
