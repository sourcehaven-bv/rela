---
id: TKT-J4HMZ2B
type: ticket
title: 'rela test v1 (1/4): untagged in-memory appbuild recipe with MemFS write / disk config split'
kind: enhancement
priority: medium
effort: l
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Description

Part 1 of TKT-0MCLX9S. The design is in PLAN-G8YMBDB and its review responses.

Add an untagged `appbuild.NewInMemory(root, opts...)`: memstore +
`search.LinearSearch`, `Config.FS = storage.NewMemFS()`, `projectConfig =
rootfs.New(root)`, in-memory audit, full `prepare` + `assemble`.
`appbuild_memory.go` `New` delegates to it. Every write (state KV, migration
record, comments, piles, leases, lock files, data-migration bootstrap) lands in
the MemFS or memory.

## Acceptance criteria

- Building a world on a project with comments, piles and a pending data migration, then writing through the entity manager, leaves the project tree byte-identical (hash before and after).
- `go list -deps -tags postgres ./cmd/rela` still links no bleve; a CI dependency-gate line covers the new recipe.
- Building and closing 100 worlds leaves no goroutines behind.
- Writes through the world run automations, validation and ACL like the default build.
