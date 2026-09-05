---
id: TKT-CFGSQL
type: ticket
title: A SQLite-backed config source, layered behind the project's files
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

The third foundation piece: a `config.Loader` reading the `project_files`
table, wired in BEHIND the filesystem loader so a project that has both keeps
behaving exactly as it did.

`sqlitestore.ProjectFiles` implements `Load`/`List` over the table added in
TKT-S1EVV7, plus `Put`/`Paths` for the `db load` and `db dump` commands that
follow. `appbuild.layerStoreConfig` composes it behind
`config.NewFSLoader` via `config.NewLayered`, in a build-tagged file so no
other build links it.

## Two constraints that shaped the design

**arch-lint forbids a store importing `internal/config`**, and Go matches
method sets exactly — so the wiring site's type assertion is on a *return
type*, and the store cannot name `config.Loader` to satisfy it. `sqlitestore`
therefore declares its own identical two-method `ConfigReader`, and a
compile-time assertion in `appbuild` pins the two equal. One duplicated
interface is the cost of both rules holding at once.

**The accessor has to be on `Store`, not just `Conn`.** It was on `Conn` alone
at first, which compiles, passes every loader test, and silently never
installs the layer — the assertion just fails and the disk-only loader is
used. `TestSQLiteStoreSatisfiesConfigProvider` exists because that failure is
otherwise invisible.

## Read/write split

`ProjectFiles()` returns the read-only `ConfigReader`; `ProjectFilesStore()`
returns the concrete type with `Put`. A config consumer cannot reach `Put` by
accident, matching how every other seam here separates its halves.

## Acceptance

- `ProjectFiles` satisfies `config.Loader` structurally (compile-time
  assertion, since the match cannot be declared).
- An absent row is `fs.ErrNotExist`-compatible — a layered loader falls
  through on exactly that error and nothing else.
- `List` is sorted, scoped, non-recursive, and treats the directory as a
  literal (a `LIKE`/`GLOB` implementation would let `a_b` match `axb`).
- An absent directory lists empty, matching the filesystem loader.
- Both backends accept and reject exactly the same names.
- Disk wins per file; `List` unions both layers.
