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

## Three packages, one file

A rela database file holds two unrelated things — the entity graph and the
operator's config — and neither should own the other, or the file they share.
So opening moved out of the store entirely:

```go
db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: p})  // owns the FILE
cfg, err := configsql.New(db.DB())                        // reads config
st,  err := sqlitestore.New(db)                           // reads the graph
```

- **`internal/sqlitedb`** owns the file: opening, PRAGMA verification, the
  single-writer lock, the schema and its migration ladder. It is not under
  `internal/store`, because owning a file is not a storage-backend concern.
- **`internal/config/configsql`** reads config from `project_files`. It
  touches no entity, relation or graph — just rows keyed by path — and imports
  `internal/config` directly, so it *declares* `config.Loader` conformance
  rather than matching it structurally.
- **`internal/store/sqlitestore`** keeps only the graph. It borrows the
  handle; `Store.Close` no longer closes the database, and the recipe that
  opened the file is what closes it.

The first draft put config inside `sqlitestore`, which was wrong: arch-lint
forbids a store importing `internal/config`, and working around that meant a
duplicated `ConfigReader` interface plus a test to keep the two in sync. That
was the design telling me the code was in the wrong package — the store's own
arch-lint comment calls it "the conformance-passing minimal store", and config
had no business widening it. Moving the package deleted the duplicate
interface, the structural-conformance test, and the build-tagged
`layerStoreConfig` indirection.

## Wiring

`appbuild`'s sqlite recipe opens the database, hands the handle to both, and
layers the config: `config.NewLayered(files, baked)`. `assemble` takes an
optional `projectConfig` override — nil on every other build, so they are
byte-identical to before.

## Acceptance

- `configsql.Loader` implements `config.Loader` (declared, compiler-checked).
- An absent row is `fs.ErrNotExist`-compatible — a layered loader falls
  through on exactly that error and nothing else.
- `List` is sorted, scoped, non-recursive, and treats the directory as a
  literal (a `LIKE`/`GLOB` implementation would let `a_b` match `axb`).
- An absent directory lists empty, matching the filesystem loader.
- Both backends accept and reject exactly the same names.
- Disk wins per file; `List` unions both layers.
