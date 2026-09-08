---
id: TKT-RAT7U3
type: ticket
title: config.Loader grows List and a disk-first layered loader
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

`config.Loader` today reads one named file. Directory-shaped config —
`scripts/`, `actions/`, `validations/`, `migrations/`, `templates/`, `custom/`,
`apps/` — has no single name, so every consumer of those trees currently
bypasses the seam with `os.OpenRoot` or `os.DirFS`.

Two additions, both confined to `internal/config`:

- `List(ctx, prefix) ([]string, error)` on the interface. `FSLoader`
implements it with `ReadDir`; a later SQLite backend with a prefix scan.
- `NewLayered(primary, fallback Loader) Loader` — disk-first, DB-fallback.

`NewLayered` is what makes the whole feature a *mode* rather than a migration:
with disk first, every call site converted in later tickets keeps behaving
exactly as it does today until someone runs `db load`. That is what lets the
per-seam conversions land independently and stay independently revertible.

`validateName` (`internal/config/config.go:85`) is already the right validator
for both backends — it rejects empty names, control characters, backslashes,
absolute paths, drive letters and `.`/`..` segments — and is reused unchanged.

## Scope

`internal/config/` only. Adding a method to an interface with a single
implementation is contained; no consumer changes.

## Acceptance

- `List` returns names sorted, relative to the prefix, non-recursive.
- A missing directory lists empty rather than erroring (mirrors
`datamigration.LoadDir`, which treats only a *missing* dir as empty and surfaces
every other error).
- `NewLayered` falls back on `os.ErrNotExist` and only on that — any other
primary error surfaces, or a transient disk fault would silently serve stale
baked config.
- Traversal rejection covered for both new entry points.
