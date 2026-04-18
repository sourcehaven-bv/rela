---
id: FEAT-CO4YP
type: feature
title: Pluggable store backends with conformance test-kit
summary: Unify entity and relation access behind `store.Store` with multiple backends (fsstore, memstore, future boltstore) sharing a single conformance test suite.
description: Pluggable store backends sit behind a single `store.Store` interface. Backend-independent invariants (ID validation, attachment keys, query semantics) live in `internal/store/storeutil` and `internal/store/storetest` so every backend runs the same conformance tests. A `Capabilities` struct lets backends opt in or out of optional features like attachments.
status: in-progress
---
