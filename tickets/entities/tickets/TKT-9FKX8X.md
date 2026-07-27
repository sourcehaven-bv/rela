---
id: TKT-9FKX8X
type: ticket
title: 'lua: reject non-string filter options in the gated rela.get_relations'
kind: enhancement
priority: medium
effort: s
status: done
---

Follow-up to RR-D7KXKV, which fixed the elevated path only and deferred this one
as a separate behavior change on a widely-used binding.

## Problem

`luaGetRelations` parsed its options with `RawGetString(...).(lua.LString)` and
discarded the `ok` result, so a non-string value dropped the filter entirely
rather than raising. `rela.get_relations({from = 12345})` returned **every**
relation the caller may see, and the script consumed that as a filtered result.

Peer-gating bounds the disclosure to the caller's own view, so this is a
correctness bug rather than an ACL hole — but the script is still silently
answering a different question than the one it asked.

Verified before the fix: all four of `{from = 12345}`, `{type = true}`, `{to =
{}}`, `{to = 0}` returned the unfiltered result.

## A live instance in our own docs

The orphan-report example in `GUIDE-scheduled-tasks.md` called
`rela.get_relations(e.id)` — a bare id, not an options table. Measured against
the live ticket graph, that returns **2246** relations where `{from = e.id}`
returns **2**, so its `#rels == 0` orphan test could never fire. The documented
scheduled task would have reported "No orphaned entities found" forever.

Fixed the example. Note this specific shape (non-table argument) is still
accepted and ignored, since `opts` is documented as optional — only the option
*values* inside the table are type-checked. The doc now calls that out.

## Resolution

`elevatedRelationQuery` was renamed `relationQuery` and is now shared by both
bindings, so the gated and elevated surfaces cannot drift on what a filter
means. They differ only in what happens afterwards: the gated reader peer-gates
the rows, the elevated one does not.

## Verification

- `TestGetRelations_RejectsNonStringFilter` — 4 subtests (number, boolean,
table, numeric-to).
- `TestGetRelations_AcceptsAbsentAndStringFilters` — 7 subtests pinning that
absent/partial/valid filters still scope correctly, so the rejection cannot
swing too far.
- `TestGetRelations_NonTableArgumentIsIgnored` — pins the deliberate asymmetry.
- Three mutations (silent-skip, reject-nil, drop-from-filter) each confirmed
failing before revert.
- All 120 live validation rules pass, including `require-relation-count.lua`,
the real in-tree caller passing `entity.id`. Every construction site builds ids
as `lua.LString`, so no caller regresses.
