---
id: TKT-YWDGZD
type: ticket
title: 'lua: rela.list_entities has no limit/paging — unbounded materialization, worsened by visibility filtering'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Summary

`rela.list_entities(type, filter?)` (`internal/lua/runtime.go:851`) takes **no
limit and no paging**. It streams every entity of a type into a Lua table in one
go. Unlike `rela.search`, which has `defaultSearchLimit` and an explicit `limit`
argument, there is no bound at all — the ceiling is "however many entities of
this type exist".

Surfaced during the TKT-ZF2DTV survey (RR-HC42R3).

## Why it matters more after TKT-ZF2DTV

Visibility filtering multiplies the peak allocation. Today the binding builds
one Lua table. After the read seam lands it additionally builds a Go slice of
the whole result set for `Reader.Filter`, plus a redacted copy per survivor —
roughly **3x peak allocation** on a large type, all live simultaneously.

TKT-ZF2DTV documents the multiplier honestly rather than hiding it, and may
chunk the filter internally, but neither fixes the underlying issue: the binding
has no way for a script to say "give me the first N".

## Scope when picked up

- Add a limit/paging surface to `list_entities`. Options to weigh:
  - a `limit` option in the existing filter table (smallest change, mirrors `search`'s `limit`),
  - a cursor/offset pair for real paging,
  - an iterator-style binding so scripts consume lazily without materializing.
- The store already exposes paged reads (`ListEntitiesPage` exists on
`store.Store` and is used elsewhere) — prefer routing to that rather than
slicing after a full scan, or the fix is cosmetic.
- Decide the default: an unbounded default is the current (dangerous) behavior;
a bounded default is a **breaking change** for scripts that rely on getting
everything, so it likely needs a migration or at least a release note.
- Interaction with visibility filtering: a page must be filled AFTER gating, or
a page of 50 can come back with 3 rows and scripts can't tell "end of data" from
"everything was hidden" — the same post-filter-pagination problem the data-entry
list pipeline already solved; reuse that reasoning.
- Same question applies to `rela.get_relations` (also unbounded, same shape).

## Non-goals

Changing `rela.search` (already bounded). Fixing this inside TKT-ZF2DTV — that
ticket documents the multiplier and moves on; this is the real fix.

## References

- `internal/lua/runtime.go:851` (`luaListEntities`), `:907` (`luaGetRelations`, same unbounded shape)
- `internal/lua/runtime.go:1366` (`luaSearch` — the bounded counterexample)
- RR-HC42R3, TKT-ZF2DTV
