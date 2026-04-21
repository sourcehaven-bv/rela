---
id: FEAT-ZPGGK
type: feature
title: Path-validation boundary via RootedFS and arch lint
summary: Introduce RootedFS as the single path-validation barrier above storage.FS, migrate callers, and enforce via arch lint
description: 'Introduce a storage.RootedFS concrete type that binds a validated root directory to an underlying storage.FS. A single resolve(key) method is the path-validation barrier, visible to CodeQL and enforced via arch lint. Delivered as 4 subtickets: (1) introduce RootedFS + pilot on state.FSKV, (2) migrate fsstore write paths (closes CodeQL path-injection alerts), (3) arch lint rules banning raw os.* I/O in high-level components, (4) package split storage/raw + storage/rooted so arch lint can enforce the dependency at import level.'
priority: medium
status: proposed
---
