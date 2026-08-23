---
id: RR-8DR44S
type: review-response
title: Observers fire for rolled-back writes, leaving the search index holding a phantom entity
finding: emit correctly buffered events into txPending, but notifyPut/notifyDelete/notifyRenamed called observers immediately, inside the transaction. A rolled-back write therefore still reached the search index, which then returns a hit for an entity the store does not have — and nothing self-heals until a full reindex. Per the package's own observer doc, 'an index must use observers', so this is the mechanism that matters. pgstore gets it right (entity.go:526-539). RunTxRollbackTests passed anyway because it asserts on events and rows, never on observers.
severity: critical
resolution: Widened pendingEvents from []store.Event to []func(*Store), matching pgstore's shape — one buffer, one ordering, and it generalizes past events. All three notify functions now defer through it inside a Tx. Added storetest.RunTxObserverIsolationTests (rollback must not notify; commit must still notify, so deferring does not silently drop callbacks); verified non-vacuous by reverting (fails with 'an observer must not see a write that was rolled back').
status: addressed
---
