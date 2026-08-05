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

`rela.list_entities(type, filter?)` takes **no limit and no paging**. It streams
every entity of a type into a Lua table in one go. Unlike `rela.search`, which
has `defaultSearchLimit` and an explicit `limit` argument, there is no bound at
all — the ceiling is "however many entities of this type exist".

Surfaced during the TKT-ZF2DTV survey (RR-HC42R3).

## Why it matters more after TKT-ZF2DTV

Visibility filtering multiplies the peak allocation. Today the binding builds
one Lua table. After the read seam lands it additionally builds a Go slice of
the whole result set for `Reader.Filter`, plus a redacted copy per survivor —
roughly **3x peak allocation** on a large type, all live simultaneously.

TKT-ZF2DTV documents the multiplier honestly rather than hiding it, and may
chunk the filter internally, but neither fixes the underlying issue: the binding
has no way for a script to say "give me the first N".

## PRECEDENT: TKT-95XU13 / PR #1241 (merged) — follow this shape

The per-list render override (`lists.<id>.export_render`) solved the same
problem for a different surface and its answers should be reused rather than
re-litigated. See `internal/lua/listmode.go`, `runtime.go`
(`registerListDocumentFields`), and `internal/dataentry/export_list.go`.

**1. The Go-side seam is `Len() + At(i)`, NOT `iter.Seq`.**

```go
type ListRows interface {
    Len() int
    At(i int) *entity.Entity
}
```

The godoc gives the reason: a script may walk the set more than once, and `At`
must be repeatable for the same index. A single-shot `iter.Seq` cannot offer
that, and `count` needs a cheap length without draining. This is the option the
"iterator-style binding" bullet below should resolve to — an indexed accessor,
not a Go iterator.

**2. The Lua-side surface IS an iterator, built as a stateful closure.**

`rela.document.rows()` returns a fresh cursor per call, so `for _, row in
rela.document.rows() do` works and can run twice. Each walk re-materializes its
tables; `runtime.go` explicitly says **do not memoize**, because memoizing
reintroduces the O(n) retention the laziness exists to avoid. The trade is
deliberate: CPU, not RAM.

**3. Truncation is made VISIBLE, not silent** — `count` (rows reachable),
`total` (pre-cap), `truncated` (derived from the two, not stored, so they cannot
drift). `total` is clamped up to `count` so a caller that under-reports cannot
produce an incoherent view.

**4. The cap is applied AFTER the ACL filter and field redaction**, and the
ordering is called out as load-bearing: an override can only ever see rows that
survived every gate, and never more than the cap allows.

Point 4 **contradicts the scope note below** about filling a page after gating.
#1241 does not refill; it caps post-gate and exposes `count`/`total`/`truncated`
so the script can distinguish "hit the cap" from "that is everything". That is
simpler and has now shipped, so prefer it over page-refilling unless a concrete
requirement demands true paging. Update this ticket's scope rather than
implementing both.

Also note `listExportCap = 5000` is a `var`, not a `const`, specifically so
tests can lower it — worth copying, since a 5000-row fixture is not a test.

## Scope when picked up

- Add a limit/paging surface to `list_entities`. Options to weigh:
  - a `limit` option in the existing filter table (smallest change, mirrors `search`'s `limit`),
  - a cursor/offset pair for real paging,
  - an iterator-style binding so scripts consume lazily without materializing
— **see the precedent above: Len/At on the Go side, stateful closure on the Lua
side.**
- The store already exposes paged reads (`ListEntitiesPage` exists on
`store.Store` and is used elsewhere) — prefer routing to that rather than
slicing after a full scan, or the fix is cosmetic.
- Decide the default: an unbounded default is the current (dangerous) behavior;
a bounded default is a **breaking change** for scripts that rely on getting
everything, so it likely needs a migration or at least a release note.
**Whatever the default, expose the `count`/`total`/`truncated` triple** so a
truncated result is never silent — that is the part of #1241 most worth copying.
- Interaction with visibility filtering: see point 4 above — #1241 caps after
gating and reports the shortfall rather than refilling the page.
- Same question applies to `rela.get_relations` (also unbounded, same shape).

## Do together with TKT-FVQ4

TKT-FVQ4 rewrites the SAME two loops (`luaListEntities` at `runtime.go:888-893`,
`luaGetRelations` at `:957-963`), which today `break` on iterator error and
discard it. Confirmed empirically 2026-07-27: `get_relations` returns **0 rows
AND no error** on a failed query, indistinguishable from "no such edges".

Bounding a loop that silently truncates on error is half a fix — the script
still cannot tell a capped result from a failed one. Do both in one pass.

## Non-goals

Changing `rela.search` (already bounded). Fixing this inside TKT-ZF2DTV — that
ticket documents the multiplier and moves on; this is the real fix.

## References

- **TKT-95XU13 / PR #1241** — `internal/lua/listmode.go`,
`registerListDocumentFields` in `runtime.go`,
`internal/dataentry/export_list.go` (cap ordering, `entitySliceRows`)
- `internal/lua/runtime.go` `luaListEntities`, `luaGetRelations` (unbounded), `luaSearch` (the bounded counterexample)
- RR-HC42R3, TKT-ZF2DTV, TKT-FVQ4
