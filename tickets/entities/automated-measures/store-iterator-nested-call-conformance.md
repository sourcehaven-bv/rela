---
id: store-iterator-nested-call-conformance
type: automated-measure
title: 'Store conformance: iterators allow nested store calls under concurrency and inside Tx'
description: Concurrent iterations of every store iterator, each making a nested store call per row, must finish within a deadline on a pool smaller than the number of iterations, and inside a Tx view. Fails for any backend that keeps a connection or cursor open across yield.
kind: test
location: internal/store/storetest/nesting.go (RunIteratorNestingTests, wired into RunAll; pgstore runs it at MaxConns=2)
status: active
---

## What it checks

A `storetest` conformance test, run by every backend: several concurrent
iterations of each iterator method (`ListEntities`, `ListEntityHeaders`,
`GraphQuery`, `GraphQueryHeaders`, `ListRelations`, `SearchVisible`,
`SearchVisibleFields` with a nesting `HiddenFieldsFunc`), each making a nested
store call per row, under a bounded deadline. It also runs one iteration with
nested calls inside a `Tx` view, where a backend holding its cursor shares a
single connection with the nested call.

pgstore runs it on its 2-connection scoped pool, so more listers than
connections makes a held connection fail with `context deadline exceeded`
instead of passing.

## Why at the conformance level

The defect was a backend breaking an invariant that only the deployment exposed.
A pgstore-only test pins this backend; the conformance test pins the contract
for any future backend too.
