---
description: "Pluggable persistence layer behind a single `store.Store` interface. Current backends: `fsstore` (markdown files on disk) and `memstore` (in-memory, for tests and scripts); a `boltstore` backend is planned. Backend-independent invariants — ID validation, attachment-key rules, query semantics, relation integrity — live in `internal/store/storeutil` and `internal/store/storetest` so every backend runs the same conformance suite. A `Capabilities` struct lets backends opt in or out of optional features (e.g. attachments)."
id: store-backends
layer: core
package: internal/store
status: draft
summary: Pluggable store.Store backends with shared conformance test-kit
title: Store Backends
type: concept
---
